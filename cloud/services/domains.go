package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"sync"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v8/pkg/dns"
	"github.com/linode/linodego"
	"golang.org/x/exp/slices"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"

	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	rutil "github.com/linode/cluster-api-provider-linode/util/reconciler"
)

type DNSEntries struct {
	options []DNSOptions
	mux     sync.RWMutex
}

type DNSOptions struct {
	Hostname      string
	Target        string
	DNSRecordType linodego.DomainRecordType
	DNSTTLSec     int
}

// EnsureDNSEntries ensures the domainrecord on Linode Cloud Manager is created, updated, or deleted based on operation passed
func EnsureDNSEntries(ctx context.Context, mscope *scope.MachineScope, operation string) error {
	// Check if instance is a control plane node
	if !kutil.IsControlPlaneMachine(mscope.Machine) {
		return nil
	}

	// Get the public IP that was assigned
	var dnss DNSEntries
	dnsEntries, err := dnss.getDNSEntriesToEnsure(mscope)
	if err != nil {
		return err
	}

	if mscope.LinodeCluster.Spec.Network.DNSProvider == "akamai" {
		return EnsureAkamaiDNSEntries(ctx, mscope, operation, dnsEntries)
	}

	return EnsureLinodeDNSEntries(ctx, mscope, operation, dnsEntries)
}

// EnsureLinodeDNSEntries ensures the domainrecord on Linode Cloud Manager is created, updated, or deleted based on operation passed
func EnsureLinodeDNSEntries(ctx context.Context, mscope *scope.MachineScope, operation string, dnsEntries []DNSOptions) error {
	// Get domainID from domain name
	domainID, err := GetDomainID(ctx, mscope)
	if err != nil {
		return err
	}

	for _, dnsEntry := range dnsEntries {
		if operation == "delete" {
			if err := DeleteDomainRecord(ctx, mscope, domainID, dnsEntry); err != nil {
				return err
			}
			continue
		}
		if err := CreateUpdateDomainRecord(ctx, mscope, domainID, dnsEntry); err != nil {
			return err
		}
	}

	return nil
}

// EnsureAkamaiDNSEntries ensures the domainrecord on Akamai EDGE DNS is created, updated, or deleted based on operation passed
func EnsureAkamaiDNSEntries(ctx context.Context, mscope *scope.MachineScope, operation string, dnsEntries []DNSOptions) error {
	linodeCluster := mscope.LinodeCluster
	linodeClusterNetworkSpec := linodeCluster.Spec.Network
	rootDomain := linodeClusterNetworkSpec.DNSRootDomain
	fqdn := linodeCluster.Name + "-" + linodeClusterNetworkSpec.DNSUniqueIdentifier + "." + rootDomain
	akaDNSClient := mscope.AkamaiDomainsClient

	for _, dnsEntry := range dnsEntries {
		recordBody, err := akaDNSClient.GetRecord(ctx, rootDomain, fqdn, string(dnsEntry.DNSRecordType))
		if err != nil {
			if !strings.Contains(err.Error(), "Not Found") {
				return err
			}
			if operation == "create" {
				if err := akaDNSClient.CreateRecord(
					ctx,
					&dns.RecordBody{
						Name:       fqdn,
						RecordType: string(dnsEntry.DNSRecordType),
						TTL:        dnsEntry.DNSTTLSec,
						Target:     []string{dnsEntry.Target},
					}, rootDomain); err != nil {
					return err
				}
			}
			continue
		}
		if operation == "delete" {
			switch {
			case len(recordBody.Target) > 1:
				recordBody.Target = removeElement(
					recordBody.Target,
					strings.Replace(dnsEntry.Target, "::", ":0:0:", 8), //nolint:mnd // 8 for 8 octest
				)
				if err := akaDNSClient.UpdateRecord(ctx, recordBody, rootDomain); err != nil {
					return err
				}
				continue
			default:
				if err := akaDNSClient.DeleteRecord(ctx, recordBody, rootDomain); err != nil {
					return err
				}
			}
		} else {
			recordBody.Target = append(recordBody.Target, dnsEntry.Target)
			if err := akaDNSClient.UpdateRecord(ctx, recordBody, rootDomain); err != nil {
				return err
			}
		}
	}
	return nil
}

func removeElement(stringList []string, elemToRemove string) []string {
	for index, element := range stringList {
		if element == elemToRemove {
			stringList = slices.Delete(stringList, index, index+1)
			continue
		}
	}
	return stringList
}

// getDNSEntriesToEnsure return DNS entries to create/delete
func (d *DNSEntries) getDNSEntriesToEnsure(mscope *scope.MachineScope) ([]DNSOptions, error) {
	d.mux.Lock()
	defer d.mux.Unlock()
	dnsTTLSec := rutil.DefaultDNSTTLSec
	if mscope.LinodeCluster.Spec.Network.DNSTTLSec != 0 {
		dnsTTLSec = mscope.LinodeCluster.Spec.Network.DNSTTLSec
	}

	if mscope.LinodeMachine.Status.Addresses == nil {
		return nil, fmt.Errorf("no addresses available on the LinodeMachine resource")
	}
	domainHostname := mscope.LinodeCluster.ObjectMeta.Name + "-" + mscope.LinodeCluster.Spec.Network.DNSUniqueIdentifier

	for _, IPs := range mscope.LinodeMachine.Status.Addresses {
		recordType := linodego.RecordTypeA
		if IPs.Type != v1beta1.MachineExternalIP {
			continue
		}
		addr, err := netip.ParseAddr(IPs.Address)
		if err != nil {
			return nil, fmt.Errorf("not a valid IP %w", err)
		}
		if !addr.Is4() {
			recordType = linodego.RecordTypeAAAA
		}
		d.options = append(d.options, DNSOptions{domainHostname, IPs.Address, recordType, dnsTTLSec})
	}
	d.options = append(d.options, DNSOptions{domainHostname, mscope.LinodeMachine.Name, linodego.RecordTypeTXT, dnsTTLSec})

	return d.options, nil
}

// GetDomainID gets the domains linode id
func GetDomainID(ctx context.Context, mscope *scope.MachineScope) (int, error) {
	rootDomain := mscope.LinodeCluster.Spec.Network.DNSRootDomain
	filter, err := json.Marshal(map[string]string{"domain": rootDomain})
	if err != nil {
		return 0, err
	}
	domains, err := mscope.LinodeDomainsClient.ListDomains(ctx, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return 0, err
	}
	if len(domains) != 1 || domains[0].Domain != rootDomain {
		return 0, fmt.Errorf("domain %s not found in list of domains owned by this account", rootDomain)
	}

	return domains[0].ID, nil
}

func CreateUpdateDomainRecord(ctx context.Context, mscope *scope.MachineScope, domainID int, dnsEntry DNSOptions) error {
	// Check if domain record exists for this IP and name combo
	filter, err := json.Marshal(map[string]interface{}{"name": dnsEntry.Hostname, "target": dnsEntry.Target, "type": dnsEntry.DNSRecordType})
	if err != nil {
		return err
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return err
	}

	// If record doesnt exist, create it
	if len(domainRecords) == 0 {
		if _, err := mscope.LinodeDomainsClient.CreateDomainRecord(
			ctx,
			domainID,
			linodego.DomainRecordCreateOptions{
				Type:   dnsEntry.DNSRecordType,
				Name:   dnsEntry.Hostname,
				Target: dnsEntry.Target,
				TTLSec: dnsEntry.DNSTTLSec,
			},
		); err != nil {
			return err
		}
		return nil
	}

	// If record exists, update it
	if len(domainRecords) != 0 && dnsEntry.DNSRecordType != linodego.RecordTypeTXT {
		isOwner, err := IsDomainRecordOwner(ctx, mscope, dnsEntry.Hostname, domainID)
		if err != nil {
			return err
		}
		if !isOwner {
			return fmt.Errorf("the domain record is not owned by this entity. wont update")
		}
	}

	if _, err := mscope.LinodeDomainsClient.UpdateDomainRecord(
		ctx,
		domainID,
		domainRecords[0].ID,
		linodego.DomainRecordUpdateOptions{
			Type:   dnsEntry.DNSRecordType,
			Name:   dnsEntry.Hostname,
			Target: dnsEntry.Target,
			TTLSec: dnsEntry.DNSTTLSec,
		},
	); err != nil {
		return err
	}
	return nil
}

func DeleteDomainRecord(ctx context.Context, mscope *scope.MachineScope, domainID int, dnsEntry DNSOptions) error {
	// Check if domain record exists for this IP and name combo
	filter, err := json.Marshal(map[string]interface{}{"name": dnsEntry.Hostname, "target": dnsEntry.Target, "type": dnsEntry.DNSRecordType})
	if err != nil {
		return err
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return err
	}

	// Nothing to do if records dont exist
	if len(domainRecords) == 0 {
		return nil
	}

	// If record is A/AAAA type, verify ownership
	if dnsEntry.DNSRecordType != linodego.RecordTypeTXT {
		isOwner, err := IsDomainRecordOwner(ctx, mscope, dnsEntry.Hostname, domainID)
		if err != nil {
			return err
		}
		if !isOwner {
			return fmt.Errorf("the domain record is not owned by this entity. wont delete")
		}
	}

	// Delete record
	if deleteErr := mscope.LinodeDomainsClient.DeleteDomainRecord(ctx, domainID, domainRecords[0].ID); deleteErr != nil {
		return deleteErr
	}
	return nil
}

func IsDomainRecordOwner(ctx context.Context, mscope *scope.MachineScope, hostname string, domainID int) (bool, error) {
	// Check if domain record exists
	filter, err := json.Marshal(map[string]interface{}{"name": hostname, "target": mscope.LinodeMachine.Name, "type": linodego.RecordTypeTXT})
	if err != nil {
		return false, err
	}

	domainRecords, err := mscope.LinodeDomainsClient.ListDomainRecords(ctx, domainID, linodego.NewListOptions(0, string(filter)))
	if err != nil {
		return false, err
	}

	// If record exists, update it
	if len(domainRecords) == 0 {
		return false, fmt.Errorf("no txt record %s found with value %s for machine %s", hostname, mscope.LinodeMachine.Name, mscope.LinodeMachine.Name)
	}

	return true, nil
}

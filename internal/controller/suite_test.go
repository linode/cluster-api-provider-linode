/*
Copyright 2023 Akamai Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	infrav2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"

	// +kubebuilder:scaffold:imports

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg               *rest.Config
	k8sClient         client.Client
	testEnv           *envtest.Environment
	clusterAPIVersion string
	data              []byte
	err               error
	_, b, _, _        = runtime.Caller(0)
	basepath          = filepath.Dir(b)
)

const (
	defaultNamespace    = "default"
	gzipCompressionFlag = false
)

func TestControllers(t *testing.T) {
	t.Parallel()

	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

func getFilePathToCAPICRDs() string {
	goModFilePath := filepath.Join(basepath, "..", "..", "go.mod")

	// Read the go.mod file
	data, err = os.ReadFile(goModFilePath)
	if err != nil {
		panic(err)
	}

	// Parse the file
	parsedFile, err := modfile.Parse(goModFilePath, data, nil)
	if err != nil {
		panic(err)
	}

	// Get the cluster-api version
	for _, goMod := range parsedFile.Require {
		if strings.Contains(goMod.Mod.Path, "cluster-api") {
			clusterAPIVersion = goMod.Mod.Version
		}
	}

	if clusterAPIVersion == "" {
		panic("Could not find cluster-api version in the go.mod file")
	}

	gopath := envOr("GOPATH", build.Default.GOPATH)
	return filepath.Join(gopath, "pkg", "mod", "sigs.k8s.io", fmt.Sprintf("cluster-api@%s", clusterAPIVersion), "config", "crd", "bases")
}

func envOr(envKey, defaultValue string) string {
	if value, ok := os.LookupEnv(envKey); ok {
		return value
	}
	return defaultValue
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	crdPaths := []string{
		filepath.Join("..", "..", "config", "crd", "bases"),
	}

	if capiPath := getFilePathToCAPICRDs(); capiPath != "" {
		crdPaths = append(crdPaths, capiPath)
	}

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     crdPaths,
		ErrorIfCRDPathMissing: true,

		// The BinaryAssetsDirectory is only required if you want to run the tests directly
		// without call the makefile target test. If not informed it will look for the
		// default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// Note that you must have the required binaries setup under the bin directory to perform
		// the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.30.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	// cfg is defined in this file globally.
	cfg, _ = testEnv.Start()
	Expect(cfg).NotTo(BeNil())

	Expect(infrav2.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(clusterv1.AddToScheme(scheme.Scheme)).To(Succeed())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

include ../../Makefile

createVPC:
	@URI=vpcs FILTER='{"label":"capli-e2e-byovpc-sample"}' make --no-print-directory callLinodeApiGet | grep -v 'results": 0' ||\
	URI=vpcs BODY='@01-vpc-create-options.json' make --no-print-directory callLinodeApiPost

enableVPC:
	PATCH='{"status":{"ready":true}}' OBJ="linodevpcs/linodevpc-sample" make --no-print-directory patchKubeObjStatus

fetchVPCID:
	@URI=vpcs FILTER='{"label":"capli-e2e-byovpc-sample"}' make --no-print-directory callLinodeApiGet | jq -r .data[0].id

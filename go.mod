module github.com/polarsignals/profile-exporter

go 1.21

toolchain go1.21.0

require (
	buf.build/gen/go/prometheus/prometheus/protocolbuffers/go v1.31.0-20230726221845-41588ce133c8.1
	github.com/alecthomas/kong v0.8.0
	github.com/apache/arrow/go/v14 v14.0.0-20230922164031-772a01c080ad
	github.com/gogo/protobuf v1.3.2
	github.com/golang/snappy v0.0.4
	github.com/oklog/run v1.1.0
	github.com/parca-dev/parca v0.19.1-0.20230926162400-7c964d9f36a9
	github.com/prometheus/common v0.44.0
	github.com/prometheus/common/sigv4 v0.1.0
	github.com/prometheus/prometheus v0.47.0
	golang.org/x/exp v0.0.0-20230905200255-921286631fa9
	google.golang.org/grpc v1.58.1
	google.golang.org/protobuf v1.31.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	buf.build/gen/go/gogo/protobuf/protocolbuffers/go v1.31.0-20210810001428-4df00b267f94.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.7.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.3.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.3.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.0.0 // indirect
	github.com/aws/aws-sdk-go v1.44.302 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/go-ozzo/ozzo-validation/v4 v4.3.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/flatbuffers v23.5.26+incompatible // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.18.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/procfs v0.11.1 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	golang.org/x/crypto v0.13.0 // indirect
	golang.org/x/mod v0.12.0 // indirect
	golang.org/x/net v0.15.0 // indirect
	golang.org/x/oauth2 v0.12.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/tools v0.13.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230803162519-f966b187b2e5 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230911183012-2d3300fd4832 // indirect
)

// required by https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v2.10.3
replace cloud.google.com/go/storage v1.19.0 => cloud.google.com/go/storage v1.10.0

module github.com/kayac/ecspresso/v2

go 1.25.0

require (
	github.com/Songmu/prompter v0.5.1
	github.com/Songmu/skillsmith v0.1.0
	github.com/alecthomas/kong v1.15.0
	github.com/aws/aws-sdk-go-v2 v1.42.0
	github.com/aws/aws-sdk-go-v2/config v1.32.17
	github.com/aws/aws-sdk-go-v2/credentials v1.19.16
	github.com/aws/aws-sdk-go-v2/service/applicationautoscaling v1.41.16
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.74.0
	github.com/aws/aws-sdk-go-v2/service/codedeploy v1.35.15
	github.com/aws/aws-sdk-go-v2/service/ecr v1.57.2
	github.com/aws/aws-sdk-go-v2/service/ecs v1.85.0
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.54.12
	github.com/aws/aws-sdk-go-v2/service/iam v1.53.10
	github.com/aws/aws-sdk-go-v2/service/lambda v1.90.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.101.0
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.41.7
	github.com/aws/aws-sdk-go-v2/service/servicediscovery v1.39.28
	github.com/aws/aws-sdk-go-v2/service/ssm v1.68.6
	github.com/aws/aws-sdk-go-v2/service/sts v1.42.1
	github.com/aws/aws-sdk-go-v2/service/vpclattice v1.21.0
	github.com/aws/smithy-go v1.27.1
	github.com/fatih/color v1.19.0
	github.com/fujiwara/cfn-lookup v1.1.0
	github.com/fujiwara/ecsta v0.8.2
	github.com/fujiwara/sloghandler v0.1.0
	github.com/fujiwara/ssm-lookup v0.1.1
	github.com/fujiwara/tfstate-lookup v1.12.1
	github.com/goccy/go-yaml v1.19.2
	github.com/google/go-cmp v0.7.0
	github.com/google/go-jsonnet v0.22.0
	github.com/hashicorp/go-envparse v0.1.0
	github.com/hashicorp/go-version v1.9.0
	github.com/hexops/gotextdiff v1.0.3
	github.com/itchyny/gojq v0.12.19
	github.com/kayac/go-config v0.7.0
	github.com/kylelemons/godebug v1.1.0
	github.com/mattn/go-isatty v0.0.22
	github.com/mattn/go-shellwords v1.0.13
	github.com/olekukonko/tablewriter v1.1.4
	github.com/opencontainers/image-spec v1.1.1
	github.com/samber/lo v1.53.0
	github.com/schollz/progressbar/v3 v3.19.0
	github.com/shogo82148/go-retry v1.3.1
	golang.org/x/sys v0.45.0
)

require (
	cel.dev/expr v0.25.2 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.20.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.7.0 // indirect
	cloud.google.com/go/monitoring v1.24.3 // indirect
	cloud.google.com/go/storage v1.62.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.20.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.13.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage v1.8.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.4 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.22 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.7 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/mocks v0.4.2 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.6.0 // indirect
	github.com/BurntSushi/toml v1.3.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.33.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.55.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.55.0 // indirect
	github.com/Songmu/flextime v0.1.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.10 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.29 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.29 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.71.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/sns v1.39.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.21 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/creack/pty v1.1.24 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.37.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fujiwara/tracer v1.1.3 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.15 // indirect
	github.com/googleapis/gax-go/v2 v2.22.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/go-slug v0.16.8 // indirect
	github.com/hashicorp/go-tfe v1.103.0 // indirect
	github.com/hashicorp/jsonapi v1.4.3-0.20250220162346-81a76b606f3e // indirect
	github.com/itchyny/timefmt-go v0.1.8 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-runewidth v0.0.23 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/olekukonko/cat v0.0.0-20250911104152-50322a0618f6 // indirect
	github.com/olekukonko/errors v1.2.0 // indirect
	github.com/olekukonko/ll v0.1.8 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/spiffe/go-spiffe/v2 v2.7.0 // indirect
	github.com/tkuchiki/go-timezone v0.2.3 // indirect
	github.com/tkuchiki/parsetime v0.3.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.44.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.67.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.67.0 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/term v0.43.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/api v0.277.0 // indirect
	google.golang.org/genproto v0.0.0-20260319201613-d00831a3d3e7 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.83.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

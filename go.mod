module github.com/kayac/ecspresso/v2

go 1.18

require (
	github.com/Songmu/prompter v0.5.1
	github.com/alecthomas/kong v0.8.1
	github.com/aws/aws-sdk-go-v2 v1.24.0
	github.com/aws/aws-sdk-go-v2/config v1.26.0
	github.com/aws/aws-sdk-go-v2/credentials v1.16.13
	github.com/aws/aws-sdk-go-v2/service/applicationautoscaling v1.25.4
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.29.4
	github.com/aws/aws-sdk-go-v2/service/codedeploy v1.22.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.24.4
	github.com/aws/aws-sdk-go-v2/service/ecs v1.35.4
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.26.4
	github.com/aws/aws-sdk-go-v2/service/iam v1.28.4
	github.com/aws/aws-sdk-go-v2/service/s3 v1.47.4
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.25.4
	github.com/aws/aws-sdk-go-v2/service/servicediscovery v1.27.4
	github.com/aws/aws-sdk-go-v2/service/ssm v1.44.4
	github.com/aws/aws-sdk-go-v2/service/sts v1.26.6
	github.com/aws/smithy-go v1.19.0
	github.com/fatih/color v1.16.0
	github.com/fujiwara/cfn-lookup v1.0.0
	github.com/fujiwara/ecsta v0.4.1
	github.com/fujiwara/logutils v1.1.2
	github.com/fujiwara/tfstate-lookup v1.1.6
	github.com/goccy/go-yaml v1.9.5
	github.com/google/go-cmp v0.5.9
	github.com/google/go-jsonnet v0.19.1
	github.com/hashicorp/go-envparse v0.1.0
	github.com/hashicorp/go-version v1.6.0
	github.com/hexops/gotextdiff v1.0.3
	github.com/itchyny/gojq v0.12.11
	github.com/kayac/go-config v0.6.0
	github.com/kylelemons/godebug v1.1.0
	github.com/mattn/go-isatty v0.0.20
	github.com/olekukonko/tablewriter v0.0.5
	github.com/opencontainers/image-spec v1.0.2
	github.com/samber/lo v1.36.0
	github.com/schollz/progressbar/v3 v3.13.1
	github.com/shogo82148/go-retry v1.1.1
	golang.org/x/sys v0.15.0
)

require (
	cloud.google.com/go v0.110.0 // indirect
	cloud.google.com/go/compute v1.19.1 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/iam v0.13.0 // indirect
	cloud.google.com/go/storage v1.28.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.7.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.3.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.3.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage v1.3.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.1.0 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.21 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.6 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/mocks v0.4.2 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.1.1 // indirect
	github.com/BurntSushi/toml v1.2.0 // indirect
	github.com/Songmu/flextime v0.1.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.14.10 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.15.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.2.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.5.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.7.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.2.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.42.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.10.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.2.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.10.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.16.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/sns v1.26.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.18.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.21.5 // indirect
	github.com/creack/pty v1.1.20 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/fujiwara/tracer v1.0.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.3 // indirect
	github.com/googleapis/gax-go/v2 v2.7.1 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.1 // indirect
	github.com/hashicorp/go-slug v0.10.0 // indirect
	github.com/hashicorp/go-tfe v1.10.0 // indirect
	github.com/hashicorp/jsonapi v0.0.0-20210826224640-ee7dae0fb22d // indirect
	github.com/itchyny/timefmt-go v0.1.5 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.4.2 // indirect
	github.com/tkuchiki/go-timezone v0.2.2 // indirect
	github.com/tkuchiki/parsetime v0.3.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/exp v0.0.0-20220303212507-bbda1eaf7a17 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/oauth2 v0.7.0 // indirect
	golang.org/x/term v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.0.0-20220722155302-e5dcc9cfc0b9 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/api v0.114.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.56.3 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

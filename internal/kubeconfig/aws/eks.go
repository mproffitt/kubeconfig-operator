package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	credsv2 "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/mproffitt/kubeconfig-operator/internal/helpers"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	LOCASTACK_DOMAIN        = "localhost.localstack.cloud"
	LOCALSTACK_ENDPOINT     = "http://localhost.localstack.cloud:4566"
	LOCALSTACK_ACCESS_TOKEN = "test"
	LOCALSTACK_SECRET_TOKEN = "test"
	DEFAULT_REGION          = "us-east-1"

	kindExecCredential     = "ExecCredential"
	presignedURLExpiration = 15 * time.Minute
	clusterIDHeader        = "x-k8s-aws-id"
	v1Prefix               = "k8s-aws-v1."
)

func KubeConfig(context string, config *rest.Config) (cfg *api.Config, err error) {
	var token string
	if token, err = getToken(context, config.Host); err != nil {
		return nil, err
	}

	cfg = &api.Config{
		APIVersion: api.SchemeGroupVersion.Version,
		Clusters: map[string]*api.Cluster{
			context: {
				Server:                   config.Host,
				CertificateAuthorityData: config.CAData,
			},
		},
		Contexts: map[string]*api.Context{
			context: {
				Cluster:  context,
				AuthInfo: config.Username,
			},
		},
		CurrentContext: context,
		AuthInfos: map[string]*api.AuthInfo{
			config.Username: {
				Token: token,
			},
		},
	}

	return cfg, nil
}

func getToken(arn, host string) (string, error) {
	var (
		err    error
		client *sts.PresignClient
	)

	awsArn := NewArn(arn)

	stsc, err := stsclient(awsArn.Region, host)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create AWS STS client")
	}

	client = presignclient(stsc)

	getCallerIdentity, err := client.PresignGetCallerIdentity(
		context.TODO(),
		&sts.GetCallerIdentityInput{},
		func(presignOptions *sts.PresignOptions) {
			presignOptions.ClientOptions = append(presignOptions.ClientOptions, func(stsOptions *sts.Options) {
				// Add clusterId Header
				stsOptions.APIOptions = append(
					stsOptions.APIOptions,
					smithyhttp.SetHeaderValue(clusterIDHeader, awsArn.ResourceName),
				)
				// Add back useless X-Amz-Expires query param
				exp := fmt.Sprintf("%d", int(presignedURLExpiration.Minutes()))
				stsOptions.APIOptions = append(stsOptions.APIOptions, smithyhttp.SetHeaderValue("X-Amz-Expires", exp))
			})
		})
	if err != nil {
		return "", errors.Wrap(err, "Failed to presign GetCallerIdentity")
	}

	token := base64.RawURLEncoding.EncodeToString([]byte(getCallerIdentity.URL))
	return formatJSON(fmt.Sprintf("%s%s", v1Prefix, token)), nil
}

func stsclient(region, host string) (*sts.Client, error) {
	var (
		accesskey                      = os.Getenv("AWS_ACCESS_KEY_ID")
		endpoint                       = os.Getenv("AWS_ENDPOINT")
		localstackHost                 = os.Getenv("LOCALSTACK_HOST")
		secretkey                      = os.Getenv("AWS_SECRET_ACCESS_KEY")
		session                        = os.Getenv("AWS_SESSION_TOKEN")
		ctx            context.Context = context.TODO()
		opts           []config.LoadOptionsFunc
		err            error
	)

	if region == "" {
		region = DEFAULT_REGION
	}

	if localstackHost == "" {
		localstackHost = LOCALSTACK_ENDPOINT
	}

	_, localstackHost, _, err = helpers.AddressToSchemeHostPort(localstackHost)
	if err != nil {
		err = errors.Wrap(err, "Failed to parse localstack host")
		return nil, err
	}

	_, host, _, err = helpers.AddressToSchemeHostPort(host)
	if err != nil {
		err = errors.Wrap(err, "Failed to parse host")
		return nil, err
	}

	if host == localstackHost {
		if endpoint == "" {
			endpoint = LOCALSTACK_ENDPOINT
		}
		if accesskey == "" {
			accesskey = LOCALSTACK_ACCESS_TOKEN
		}
		if secretkey == "" {
			secretkey = LOCALSTACK_SECRET_TOKEN
		}
	}

	opts = append(opts, config.WithRegion(region))
	opts = append(opts, config.WithCredentialsProvider(
		credsv2.NewStaticCredentialsProvider(accesskey, secretkey, session),
	))

	var cfg aws.Config
	if cfg, err = config.LoadDefaultConfig(
		ctx, func(cfg *config.LoadOptions) error {
			for _, opt := range opts {
				if err := opt(cfg); err != nil {
					return err
				}
			}
			return nil
		},
	); err != nil {
		return nil, errors.Wrap(err, "Failed to load AWS config")
	}

	client := sts.NewFromConfig(cfg, func(o *sts.Options) {
		o.EndpointResolverV2 = &resolverV2{
			Endpoint: endpoint,
		}
	})
	return client, err
}

func presignclient(client *sts.Client) *sts.PresignClient {
	var presign *sts.PresignClient = sts.NewPresignClient(client)

	return presign
}

func formatJSON(token string) string {
	expTime := time.Now().Local().Add(presignedURLExpiration - 1*time.Minute)
	expirationTimestamp := metav1.NewTime(expTime)

	apiVersion := clientauthv1beta1.SchemeGroupVersion.String()
	execObj := &clientauthv1beta1.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kindExecCredential,
		},
		Status: &clientauthv1beta1.ExecCredentialStatus{
			ExpirationTimestamp: &expirationTimestamp,
			Token:               token,
		},
	}

	jsonData, _ := json.Marshal(execObj)
	return string(jsonData)
}

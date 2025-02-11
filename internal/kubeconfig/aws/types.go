package aws

import (
	"context"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
)

type awsArn struct {
	Partition    string
	Service      string
	Region       string
	AccountID    string
	Resource     string
	ResourceName string
}

func NewArn(arn string) *awsArn {
	var (
		parts                  = strings.Split(arn, ":")
		resourceParts          []string
		resource, resourceName string
	)

	resourceParts = strings.Split(parts[5], "/")
	if len(resourceParts) > 1 {
		resource = resourceParts[0]
		resourceName = resourceParts[1]
	}

	return &awsArn{
		Partition:    parts[1],
		Service:      parts[2],
		Region:       parts[3],
		AccountID:    parts[4],
		Resource:     resource,
		ResourceName: resourceName,
	}
}

type resolverV2 struct {
	Endpoint string
}

func (r *resolverV2) ResolveEndpoint(ctx context.Context, params sts.EndpointParameters) (
	smithyendpoints.Endpoint, error,
) {
	if r.Endpoint != "" {
		u, err := url.Parse(r.Endpoint)
		if err != nil {
			return smithyendpoints.Endpoint{}, err
		}

		return smithyendpoints.Endpoint{
			URI: *u,
		}, nil
	}
	return sts.NewDefaultEndpointResolverV2().ResolveEndpoint(ctx, params)
}

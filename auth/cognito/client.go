package cognito

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
)

// NewIdentityProvider builds a Cognito Identity Provider client.
// Anonymous credentials are used for USER_SRP_AUTH / USER_PASSWORD_AUTH
// (the same approach as jwt-token-printer). A custom Endpoint is honoured
// so cognito-local can be used without hitting AWS.
func NewIdentityProvider(ctx context.Context, cfg Config) (*cip.Client, error) {
	cfg.Normalize()
	if cfg.Region == "" {
		return nil, fmt.Errorf("cognito region is required")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	opts := []func(*cip.Options){}
	if cfg.Endpoint != "" {
		endpoint := cfg.Endpoint
		opts = append(opts, func(o *cip.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}

	return cip.NewFromConfig(awsCfg, opts...), nil
}

// SecretHash is the Cognito SECRET_HASH: Base64(HMAC_SHA256(username+clientId, clientSecret)).
func SecretHash(username, clientID, clientSecret string) string {
	mac := hmac.New(sha256.New, []byte(clientSecret))
	_, _ = mac.Write([]byte(username + clientID))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

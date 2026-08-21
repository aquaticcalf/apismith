package cognito

import (
	"context"
	"fmt"
	"time"

	cognitosrp "github.com/alexrudd/cognito-srp/v4"
	"github.com/aws/aws-sdk-go-v2/aws"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

// Tokens is the set returned by a successful Cognito authentication.
// The API backend verifies access tokens (token_use=access), so AccessToken
// is what the console attaches as Authorization: Bearer.
type Tokens struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int32  `json:"expires_in"`
}

// Generate authenticates against Cognito and returns JWTs.
func Generate(ctx context.Context, cfg Config) (*Tokens, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := NewIdentityProvider(ctx, cfg)
	if err != nil {
		return nil, err
	}

	switch cfg.AuthFlow {
	case AuthFlowPassword:
		return generatePassword(ctx, client, cfg)
	default:
		return generateSRP(ctx, client, cfg)
	}
}

func generateSRP(ctx context.Context, client *cip.Client, cfg Config) (*Tokens, error) {
	var secret *string
	if cfg.ClientSecret != "" {
		secret = aws.String(cfg.ClientSecret)
	}

	csrp, err := cognitosrp.NewCognitoSRP(cfg.Username, cfg.Password, cfg.UserPoolID, cfg.ClientID, secret)
	if err != nil {
		return nil, fmt.Errorf("init SRP: %w", err)
	}

	resp, err := client.InitiateAuth(ctx, &cip.InitiateAuthInput{
		AuthFlow:       types.AuthFlowTypeUserSrpAuth,
		ClientId:       aws.String(csrp.GetClientId()),
		AuthParameters: csrp.GetAuthParams(),
	})
	if err != nil {
		return nil, fmt.Errorf("initiate SRP auth: %w", err)
	}

	if resp.ChallengeName != types.ChallengeNameTypePasswordVerifier {
		if resp.ChallengeName == "" && resp.AuthenticationResult != nil {
			return tokensFromResult(resp.AuthenticationResult)
		}
		return nil, fmt.Errorf("unsupported Cognito challenge %q (expected PASSWORD_VERIFIER)", resp.ChallengeName)
	}

	challengeResponses, err := csrp.PasswordVerifierChallenge(resp.ChallengeParameters, time.Now())
	if err != nil {
		return nil, fmt.Errorf("SRP password verifier: %w", err)
	}

	challenge, err := client.RespondToAuthChallenge(ctx, &cip.RespondToAuthChallengeInput{
		ChallengeName:      types.ChallengeNameTypePasswordVerifier,
		ChallengeResponses: challengeResponses,
		ClientId:           aws.String(csrp.GetClientId()),
	})
	if err != nil {
		return nil, fmt.Errorf("respond to SRP challenge: %w", err)
	}
	if challenge.AuthenticationResult == nil {
		return nil, fmt.Errorf("Cognito returned no authentication result (challenge=%q)", challenge.ChallengeName)
	}
	return tokensFromResult(challenge.AuthenticationResult)
}

func generatePassword(ctx context.Context, client *cip.Client, cfg Config) (*Tokens, error) {
	params := map[string]string{
		"USERNAME": cfg.Username,
		"PASSWORD": cfg.Password,
	}
	if cfg.ClientSecret != "" {
		params["SECRET_HASH"] = SecretHash(cfg.Username, cfg.ClientID, cfg.ClientSecret)
	}

	resp, err := client.InitiateAuth(ctx, &cip.InitiateAuthInput{
		AuthFlow:       types.AuthFlowTypeUserPasswordAuth,
		ClientId:       aws.String(cfg.ClientID),
		AuthParameters: params,
	})
	if err != nil {
		return nil, fmt.Errorf("initiate password auth: %w", err)
	}
	if resp.AuthenticationResult == nil {
		return nil, fmt.Errorf("Cognito returned no authentication result (challenge=%q)", resp.ChallengeName)
	}
	return tokensFromResult(resp.AuthenticationResult)
}

func tokensFromResult(result *types.AuthenticationResultType) (*Tokens, error) {
	if result == nil {
		return nil, fmt.Errorf("empty authentication result")
	}
	out := &Tokens{
		ExpiresIn: result.ExpiresIn,
		TokenType: aws.ToString(result.TokenType),
	}
	if result.AccessToken != nil {
		out.AccessToken = *result.AccessToken
	}
	if result.IdToken != nil {
		out.IDToken = *result.IdToken
	}
	if result.RefreshToken != nil {
		out.RefreshToken = *result.RefreshToken
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("Cognito returned an empty access token")
	}
	if out.TokenType == "" {
		out.TokenType = "Bearer"
	}
	return out, nil
}

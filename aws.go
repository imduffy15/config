package config

import (
	"context"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/secretsmanageriface"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/ssmiface"
	"github.com/pkg/errors"
)

var (
	secretsManagerStringRe = regexp.MustCompile("^sm://")
	parameterStoreStringRe = regexp.MustCompile("^ssm://")
)

func checkPrefixAndStrip(re *regexp.Regexp, s string) (string, bool) {
	if re.MatchString(s) {
		return re.ReplaceAllString(s, ""), true
	}
	return s, false
}

// NewAWSSecretManagerValuePreProcessor creates a new AWSSecretManagerValuePreProcessor with the given context and whether to decrypt parameter store values or not.
// This will load the aws config from external.LoadDefaultAWSConfig()
func NewAWSSecretManagerValuePreProcessor(ctx context.Context, decryptParameterStoreValues bool) (*AWSSecretManagerValuePreProcessor, error) {
	awsConfig, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return nil, errors.Wrap(err, "config/aws: error loading default aws config")
	}

	return &AWSSecretManagerValuePreProcessor{
		decryptParameterStoreValues: decryptParameterStoreValues,

		secretsManager: secretsmanager.New(awsConfig),
		parameterStore: ssm.New(awsConfig),
		ctx:            ctx,
	}, nil
}

// AWSSecretManagerValuePreProcessor is a ValuePreProcessor for AWS.
// Supports Secrets Manager and Parameter Store.
type AWSSecretManagerValuePreProcessor struct {
	decryptParameterStoreValues bool

	secretsManager secretsmanageriface.ClientAPI
	parameterStore ssmiface.ClientAPI
	ctx            context.Context
}

// PreProcessValue pre-processes a config key/value pair.
func (p *AWSSecretManagerValuePreProcessor) PreProcessValue(key, value string) string {
	return p.processConfigItem(p.ctx, key, value)
}

func (p *AWSSecretManagerValuePreProcessor) processConfigItem(ctx context.Context, key string, value string) string {
	if v, ok := checkPrefixAndStrip(secretsManagerStringRe, value); ok {
		return p.loadStringValueFromSecretsManager(ctx, v)
	} else if v, ok := checkPrefixAndStrip(parameterStoreStringRe, v); ok {
		return p.loadStringValueFromParameterStore(ctx, v, p.decryptParameterStoreValues)
	}
	return value
}

func (p *AWSSecretManagerValuePreProcessor) loadStringValueFromSecretsManager(ctx context.Context, name string) string {
	resp, err := p.requestSecret(ctx, name)
	if err != nil {
		panic("config/aws/loadStringValueFromSecretsManager: error loading secret, " + err.Error())
	}

	return *resp.SecretString
}

func (p *AWSSecretManagerValuePreProcessor) requestSecret(ctx context.Context, name string) (*secretsmanager.GetSecretValueResponse, error) {
	input := &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)}
	return p.secretsManager.GetSecretValueRequest(input).Send(ctx)
}

func (p *AWSSecretManagerValuePreProcessor) loadStringValueFromParameterStore(ctx context.Context, name string, decrypt bool) string {
	resp, err := p.requestParameter(ctx, name, decrypt)
	if err != nil {
		panic("config/aws/loadStringValueFromParameterStore: error loading value, " + err.Error())
	}

	return *resp.Parameter.Value
}

func (p *AWSSecretManagerValuePreProcessor) requestParameter(ctx context.Context, name string, decrypt bool) (*ssm.GetParameterResponse, error) {
	input := &ssm.GetParameterInput{Name: aws.String(name), WithDecryption: aws.Bool(decrypt)}
	return p.parameterStore.GetParameterRequest(input).Send(ctx)
}

// compile time assertion
var _ ValuePreProcessor = (*AWSSecretManagerValuePreProcessor)(nil)

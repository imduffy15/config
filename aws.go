package config

import (
	"context"
    "encoding/json"
    "fmt"
    "regexp"
    "strings"

    "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
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

func checkPostfixAndStrip(s string) (string, string) {
    res := strings.Split(s, "#")
    if len(res) > 1 {
        return res[0], res[1]
    } else {
        return res[0], ""
    }
}

// NewAWSSecretManagerValuePreProcessor creates a new AWSSecretManagerValuePreProcessor with the given context and whether to decrypt parameter store values or not.
// This will load the aws config from external.LoadDefaultAWSConfig()
func NewAWSSecretManagerValuePreProcessor(ctx context.Context, decryptParameterStoreValues bool) (*AWSSecretManagerValuePreProcessor, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "config/aws: error loading default aws config")
	}

	return &AWSSecretManagerValuePreProcessor{
		decryptParameterStoreValues: decryptParameterStoreValues,

		secretsManager: secretsmanager.NewFromConfig(awsConfig),
		parameterStore: ssm.NewFromConfig(awsConfig),
		ctx:            ctx,
	}, nil
}

type SecretsManager interface {
    GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type ParameterStoreManager interface {
    GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// AWSSecretManagerValuePreProcessor is a ValuePreProcessor for AWS.
// Supports Secrets Manager and Parameter Store.
type AWSSecretManagerValuePreProcessor struct {
	decryptParameterStoreValues bool

	secretsManager SecretsManager
	parameterStore ParameterStoreManager
	ctx            context.Context
}

// PreProcessValue pre-processes a config key/value pair.
func (p *AWSSecretManagerValuePreProcessor) PreProcessValue(key, value string) string {
	return p.processConfigItem(p.ctx, key, value)
}

func (p *AWSSecretManagerValuePreProcessor) processConfigItem(ctx context.Context, key string, value string) string {
	if v, ok := checkPrefixAndStrip(secretsManagerStringRe, value); ok {
	    v, subKey := checkPostfixAndStrip(v)
		secret := p.loadStringValueFromSecretsManager(ctx, v)
		if subKey == "" {
		    return secret
        } else {
            jsonMap := make(map[string]string)
            err := json.Unmarshal([]byte(secret), &jsonMap)
            if err != nil {
                panic("config/aws/loadStringValueFromSecretsManager: error parsing secret map, " + err.Error())
            }
            if subkeySecret, ok := jsonMap[subKey]; ok {
                return subkeySecret
            } else {
                panic(fmt.Sprintf("config/aws/loadStringValueFromSecretsManager: failed to find subkey %s", subKey))
            }
        }
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

func (p *AWSSecretManagerValuePreProcessor) requestSecret(ctx context.Context, name string) (*secretsmanager.GetSecretValueOutput, error) {
	return p.secretsManager.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
}

func (p *AWSSecretManagerValuePreProcessor) loadStringValueFromParameterStore(ctx context.Context, name string, decrypt bool) string {
	resp, err := p.requestParameter(ctx, name, decrypt)

	if err != nil {
		panic("config/aws/loadStringValueFromParameterStore: error loading value, " + err.Error())
	}

	return *resp.Parameter.Value
}

func (p *AWSSecretManagerValuePreProcessor) requestParameter(ctx context.Context, name string, decrypt bool) (*ssm.GetParameterOutput, error) {
	return p.parameterStore.GetParameter(ctx, &ssm.GetParameterInput{
	    Name: aws.String(name),
	    WithDecryption: decrypt,
    })
}

// compile time assertion
var _ ValuePreProcessor = (*AWSSecretManagerValuePreProcessor)(nil)

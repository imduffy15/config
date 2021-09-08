package config

import (
    "context"
    "encoding/base64"
    "github.com/aws/aws-sdk-go-v2/service/ssm/types"
    "github.com/aws/smithy-go/middleware"
    "testing"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
    "github.com/aws/aws-sdk-go-v2/service/ssm"
    "github.com/stretchr/testify/assert"
)

type mockSecretManagerClient struct {
    checkInput  func(*secretsmanager.GetSecretValueInput)
    stringValue *string
    binaryValue []byte
}

func (m *mockSecretManagerClient) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
    if m.checkInput != nil {
        m.checkInput(params)
    }

    return &secretsmanager.GetSecretValueOutput{
        ARN:            nil,
        CreatedDate:    nil,
        Name:           nil,
        SecretBinary:   m.binaryValue,
        SecretString:   m.stringValue,
        VersionId:      nil,
        VersionStages:  nil,
        ResultMetadata: middleware.Metadata{},
    }, nil
}

type mockParameterStoreClient struct {
    checkInput  func(*ssm.GetParameterInput)
    stringValue *string
    binaryValue []byte
}

func (m mockParameterStoreClient) GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
    if m.checkInput != nil {
        m.checkInput(params)
    }

    var value *string

    if m.stringValue != nil {
        value = m.stringValue
    } else if m.binaryValue != nil {
        value = aws.String(base64.StdEncoding.EncodeToString(m.binaryValue))
    }

    return &ssm.GetParameterOutput{
        Parameter: &types.Parameter{
            Value: value,
        },
        ResultMetadata: middleware.Metadata{},
    }, nil
}

func TestAWSSecretManagerValuePreProcessor_PreProcessValue(t *testing.T) {
	ctx := context.Background()

	t.Run("NonPrefixedValues", func(t *testing.T) {
		p := AWSSecretManagerValuePreProcessor{}

		assert.Equal(t, "bar", p.PreProcessValue("FOO_1", "bar"))
		assert.Equal(t, "test", p.PreProcessValue("FOO_BAR_BAZ", "test"))
	})

	t.Run("SecretsManager", func(t *testing.T) {
		manager := &mockSecretManagerClient{}

		p := &AWSSecretManagerValuePreProcessor{
			decryptParameterStoreValues: true,
			secretsManager:              manager,
			ctx:                         ctx,
		}

		t.Run("Simple", func(t *testing.T) {
			manager.checkInput = func(input *secretsmanager.GetSecretValueInput) {
				assert.Equal(t, "foo_bar", *input.SecretId)
			}
			manager.stringValue = aws.String("baz")

			assert.Equal(t, "baz", p.PreProcessValue("FOO", "sm://foo_bar"))
		})

		// "complex" in the sense that this would break using strings.TrimPrefix(...)
		t.Run("Complex", func(t *testing.T) {
			manager.checkInput = func(input *secretsmanager.GetSecretValueInput) {
				assert.Equal(t, "small_foo_bar", *input.SecretId)
			}
			manager.stringValue = aws.String("baz")

			assert.Equal(t, "baz", p.PreProcessValue("FOO", "sm://small_foo_bar"))
		})
	})

	t.Run("ParameterStore", func(t *testing.T) {
		storeClient := &mockParameterStoreClient{}

		p := &AWSSecretManagerValuePreProcessor{
			decryptParameterStoreValues: true,
			parameterStore:              storeClient,
			ctx:                         ctx,
		}

		t.Run("Simple", func(t *testing.T) {
			storeClient.checkInput = func(input *ssm.GetParameterInput) {
				assert.Equal(t, "foo_bar", *input.Name)
				assert.True(t, input.WithDecryption)
			}
			storeClient.stringValue = aws.String("baz")

			assert.Equal(t, "baz", p.PreProcessValue("FOO", "ssm://foo_bar"))
		})

		// "complex" in the sense that this would break using strings.TrimPrefix(...)
		t.Run("Complex", func(t *testing.T) {
			storeClient.checkInput = func(input *ssm.GetParameterInput) {
				assert.Equal(t, "ssmall_foo_bar", *input.Name)
				assert.True(t, input.WithDecryption)
			}
			storeClient.stringValue = aws.String("baz")

			assert.Equal(t, "baz", p.PreProcessValue("FOO", "ssm://ssmall_foo_bar"))
		})
	})
}

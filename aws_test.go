package config

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/secretsmanageriface"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/ssmiface"
	"github.com/stretchr/testify/assert"
)

type mockSecretManagerClient struct {
	secretsmanageriface.ClientAPI

	checkInput  func(*secretsmanager.GetSecretValueInput)
	stringValue *string
	binaryValue []byte
}

func (m *mockSecretManagerClient) GetSecretValueRequest(in *secretsmanager.GetSecretValueInput) secretsmanager.GetSecretValueRequest {
	if m.checkInput != nil {
		m.checkInput(in)
	}

	req := &aws.Request{
		Data: &secretsmanager.GetSecretValueOutput{
			SecretString: m.stringValue,
			SecretBinary: m.binaryValue,
		},
		HTTPRequest: new(http.Request),
	}
	return secretsmanager.GetSecretValueRequest{Request: req, Input: in, Copy: m.GetSecretValueRequest}
}

type mockParameterStoreClient struct {
	ssmiface.ClientAPI

	checkInput  func(*ssm.GetParameterInput)
	stringValue *string
	binaryValue []byte
}

func (m *mockParameterStoreClient) GetParameterRequest(in *ssm.GetParameterInput) ssm.GetParameterRequest {
	if m.checkInput != nil {
		m.checkInput(in)
	}

	var value *string

	if m.stringValue != nil {
		value = m.stringValue
	} else if m.binaryValue != nil {
		value = aws.String(base64.StdEncoding.EncodeToString(m.binaryValue))
	}

	req := &aws.Request{
		Data: &ssm.GetParameterOutput{
			Parameter: &ssm.Parameter{
				Value: value,
			},
		},
		HTTPRequest: new(http.Request),
	}
	return ssm.GetParameterRequest{Request: req, Input: in, Copy: m.GetParameterRequest}
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
				assert.True(t, *input.WithDecryption)
			}
			storeClient.stringValue = aws.String("baz")

			assert.Equal(t, "baz", p.PreProcessValue("FOO", "ssm://foo_bar"))
		})

		// "complex" in the sense that this would break using strings.TrimPrefix(...)
		t.Run("Complex", func(t *testing.T) {
			storeClient.checkInput = func(input *ssm.GetParameterInput) {
				assert.Equal(t, "ssmall_foo_bar", *input.Name)
				assert.True(t, *input.WithDecryption)
			}
			storeClient.stringValue = aws.String("baz")

			assert.Equal(t, "baz", p.PreProcessValue("FOO", "ssm://ssmall_foo_bar"))
		})
	})
}

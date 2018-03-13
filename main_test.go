package main

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestConvertSecret(t *testing.T) {
	g := NewGomegaWithT(t)
	result := secretData2EnvMapping(map[string]interface{}{
		"access_key":     "abc",
		"secret_key":     "mysecret",
		"security_token": nil,
	})
	expected := map[string]string{
		"AWS_ACCESS_KEY_ID":     "abc",
		"AWS_SECRET_ACCESS_KEY": "mysecret",
	}
	g.Expect(result).Should(Equal(expected))
	result = secretData2EnvMapping(map[string]interface{}{
		"access_key":     "abc",
		"secret_key":     "mysecret",
		"security_token": "apple",
	})
	g.Expect(result).Should(Equal(map[string]string{
		"AWS_ACCESS_KEY_ID":     "abc",
		"AWS_SECRET_ACCESS_KEY": "mysecret",
		"AWS_SESSION_TOKEN":     "apple",
	}))
}

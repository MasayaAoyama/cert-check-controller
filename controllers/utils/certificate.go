package utils

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetExpirationDate(secret *v1.Secret) (metav1.Time, metav1.Time, error) {
	block, _ := pem.Decode(secret.Data[v1.TLSCertKey])
	if block == nil {
		return metav1.Now(), metav1.Now(), fmt.Errorf("failed to parse certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)

	if err != nil {
		return metav1.Now(), metav1.Now(), err
	}

	return metav1.NewTime(cert.NotBefore), metav1.NewTime(cert.NotAfter), nil
}

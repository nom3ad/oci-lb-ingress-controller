package ingress

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/nom3ad/oci-lb-ingress-controller/src/utils"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	oidExtensionSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}
)

type CertificateBundle struct {
	CertificatePem         string
	CertificateX509        x509.Certificate
	PrivateKeyPem          string
	CACertificateChainPem  *string
	CACertificateChainX509 []x509.Certificate
	Domains                []string
}

func (cd *CertificateBundle) UniqueID() string {
	sig := cd.CertificateX509.Signature
	for _, x := range cd.CACertificateChainX509 {
		sig = append(sig, x.Signature...)
	}
	return utils.ByteAlphaNumericDigest(sig, 24)
}

func (cd *CertificateBundle) Dump() string {
	asStr := func(c x509.Certificate) string {
		return fmt.Sprintf("S:%s|I:%s|%s<%s|#%s", c.Subject, c.Issuer, c.NotBefore.Format(time.RFC822), c.NotAfter.Format(time.RFC822), c.SerialNumber)
	}
	s := fmt.Sprintf("%s %s", strings.Join(cd.Domains, ","), asStr(cd.CertificateX509))
	for _, x := range cd.CACertificateChainX509 {
		s = fmt.Sprintf("%s\t%s", s, asStr(x))
	}
	return s
}

func getExtension(c *x509.Certificate, id asn1.ObjectIdentifier) []pkix.Extension {
	var exts []pkix.Extension
	for _, ext := range c.Extensions {
		if ext.Id.Equal(id) {
			exts = append(exts, ext)
		}
	}
	return exts
}

func ParseSANExtension(value []byte) (dnsNames, emailAddresses []string, ipAddresses []net.IP, err error) {
	// RFC 5280, 4.2.1.6

	// SubjectAltName ::= GeneralNames
	//
	// GeneralNames ::= SEQUENCE SIZE (1..MAX) OF GeneralName
	//
	// GeneralName ::= CHOICE {
	//      otherName                       [0]     OtherName,
	//      rfc822Name                      [1]     IA5String,
	//      dNSName                         [2]     IA5String,
	//      x400Address                     [3]     ORAddress,
	//      directoryName                   [4]     Name,
	//      ediPartyName                    [5]     EDIPartyName,
	//      uniformResourceIdentifier       [6]     IA5String,
	//      iPAddress                       [7]     OCTET STRING,
	//      registeredID                    [8]     OBJECT IDENTIFIER }
	var seq asn1.RawValue
	var rest []byte
	if rest, err = asn1.Unmarshal(value, &seq); err != nil {
		return
	} else if len(rest) != 0 {
		err = errors.New("x509: trailing data after X.509 extension")
		return
	}
	if !seq.IsCompound || seq.Tag != 16 || seq.Class != 0 {
		err = asn1.StructuralError{Msg: "bad SAN sequence"}
		return
	}

	rest = seq.Bytes
	for len(rest) > 0 {
		var v asn1.RawValue
		rest, err = asn1.Unmarshal(rest, &v)
		if err != nil {
			return
		}
		switch v.Tag {
		case 1:
			emailAddresses = append(emailAddresses, string(v.Bytes))
		case 2:
			dnsNames = append(dnsNames, string(v.Bytes))
		case 7:
			switch len(v.Bytes) {
			case net.IPv4len, net.IPv6len:
				ipAddresses = append(ipAddresses, v.Bytes)
			default:
				err = errors.New("x509: certificate contained IP address of length " + strconv.Itoa(len(v.Bytes)))
				return
			}
		}
	}

	return
}

func getCertificateBundle(ctx context.Context, namespace, secretName string, k8sClient k8sclient.Client) (*CertificateBundle, error) {
	secret := corev1.Secret{}
	secretNsName := utils.AsNamespacedName(secretName, namespace)
	if err := k8sClient.Get(ctx, secretNsName, &secret); err != nil {
		return nil, errors.Wrapf(err, "Could not get secret %s", secretNsName)
	}
	certPemStr := string(secret.Data[corev1.TLSCertKey])
	privateKeyPemStr := string(secret.Data[corev1.TLSPrivateKeyKey])
	caPemStr := string(secret.Data["ca.crt"])

	certs, err := ParseCertificatesFromPEM(certPemStr)
	certX509 := certs[0]
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse tls certificate from secret %s", secretName)
	}

	domains := sets.NewString(certX509.Subject.CommonName)
	for _, dns := range certX509.DNSNames {
		if !domains.Has(dns) {
			domains.Insert(dns)
		}
	}

	if len(certX509.Extensions) > 0 {
		for _, ext := range getExtension(&certX509, oidExtensionSubjectAltName) {
			dns, _, _, err := ParseSANExtension(ext.Value)
			if err != nil {
				continue
			}
			for _, dns := range dns {
				if !domains.Has(dns) {
					domains.Insert(dns)
				}
			}
		}
	}
	cd := CertificateBundle{
		CertificatePem:  certPemStr,
		PrivateKeyPem:   privateKeyPemStr,
		CertificateX509: certX509,
		Domains:         domains.List(),
	}
	if caPemStr != "" {
		certs, err := ParseCertificatesFromPEM(caPemStr)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse ca certificate chain from secret %s", secretName)
		}
		cd.CACertificateChainX509 = certs
		cd.CACertificateChainPem = &caPemStr
	} else if len(certs) > 1 {
		cd.CACertificateChainX509 = certs[1:]
		for _, c := range cd.CACertificateChainX509 {
			pemBlock := pem.Block{
				Type:  "CERTIFICATE",
				Bytes: c.Raw,
			}
			caPemStr += string(pem.EncodeToMemory(&pemBlock))
		}
	}
	return &cd, nil
}

func ParseCertificatesFromPEM(content string) ([]x509.Certificate, error) {
	var certs []x509.Certificate
	for {
		content = strings.TrimSpace(content)
		if content == "" {
			break
		}
		pemBlock, rest := pem.Decode([]byte(content))
		if pemBlock == nil {
			return nil, fmt.Errorf("no certificate PEM data found")
		}
		if pemBlock.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("invalid PEM type: %s", pemBlock.Type)
		}
		x509Cert, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, *x509Cert)
		content = string(rest)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates parsed")
	}
	return certs, nil
}

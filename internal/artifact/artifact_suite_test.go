package artifact_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	templateKeyfile  = "test/credentials/test-keyfile.json"
	generatedKeyfile = "test/credentials/generated-test-keyfile.json"
)

func TestArtifact(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Artifact Suite")
}

// The Google auth library rejects service account files without a private key,
// so a throwaway key is generated per run instead of committing one.
var _ = BeforeSuite(func() {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).To(BeNil())
	der, err := x509.MarshalPKCS8PrivateKey(key)
	Expect(err).To(BeNil())

	b, err := os.ReadFile(templateKeyfile)
	Expect(err).To(BeNil())

	keyfile := map[string]interface{}{}
	Expect(json.Unmarshal(b, &keyfile)).To(Succeed())
	keyfile["private_key"] = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	b, err = json.Marshal(keyfile)
	Expect(err).To(BeNil())
	Expect(os.WriteFile(generatedKeyfile, b, 0600)).To(Succeed())
	Expect(os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", generatedKeyfile)).To(Succeed())
})

var _ = AfterSuite(func() {
	os.Remove(generatedKeyfile)
})

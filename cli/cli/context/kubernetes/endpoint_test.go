package kubernetes

import (
	"os"
	"testing"

	"github.com/docker/cli/cli/context"
	"github.com/docker/cli/cli/context/store"
	"gotest.tools/v3/assert"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func testEndpoint(server, defaultNamespace string, ca, cert, key []byte, skipTLSVerify bool) Endpoint {
	var tlsData *context.TLSData
	if ca != nil || cert != nil || key != nil {
		tlsData = &context.TLSData{
			CA:   ca,
			Cert: cert,
			Key:  key,
		}
	}
	return Endpoint{
		EndpointMeta: EndpointMeta{
			EndpointMetaBase: context.EndpointMetaBase{
				Host:          server,
				SkipTLSVerify: skipTLSVerify,
			},
			DefaultNamespace: defaultNamespace,
		},
		TLSData: tlsData,
	}
}

var testStoreCfg = store.NewConfig(
	func() interface{} {
		return &map[string]interface{}{}
	},
	store.EndpointTypeGetter(KubernetesEndpoint, func() interface{} { return &EndpointMeta{} }),
)

func TestSaveLoadContexts(t *testing.T) {
	store := store.New(t.TempDir(), testStoreCfg)
	assert.NilError(t, save(store, testEndpoint("https://test", "test", nil, nil, nil, false), "raw-notls"))
	assert.NilError(t, save(store, testEndpoint("https://test", "test", nil, nil, nil, true), "raw-notls-skip"))
	assert.NilError(t, save(store, testEndpoint("https://test", "test", []byte("ca"), []byte("cert"), []byte("key"), true), "raw-tls"))

	kcFile, err := os.CreateTemp(t.TempDir(), "test-load-save-k8-context")
	assert.NilError(t, err)
	defer kcFile.Close()
	cfg := clientcmdapi.NewConfig()
	cfg.AuthInfos["user"] = clientcmdapi.NewAuthInfo()
	cfg.Contexts["context1"] = clientcmdapi.NewContext()
	cfg.Clusters["cluster1"] = clientcmdapi.NewCluster()
	cfg.Contexts["context2"] = clientcmdapi.NewContext()
	cfg.Clusters["cluster2"] = clientcmdapi.NewCluster()
	cfg.AuthInfos["user"].ClientCertificateData = []byte("cert")
	cfg.AuthInfos["user"].ClientKeyData = []byte("key")
	cfg.Clusters["cluster1"].Server = "https://server1"
	cfg.Clusters["cluster1"].InsecureSkipTLSVerify = true
	cfg.Clusters["cluster2"].Server = "https://server2"
	cfg.Clusters["cluster2"].CertificateAuthorityData = []byte("ca")
	cfg.Contexts["context1"].AuthInfo = "user"
	cfg.Contexts["context1"].Cluster = "cluster1"
	cfg.Contexts["context1"].Namespace = "namespace1"
	cfg.Contexts["context2"].AuthInfo = "user"
	cfg.Contexts["context2"].Cluster = "cluster2"
	cfg.Contexts["context2"].Namespace = "namespace2"
	cfg.CurrentContext = "context1"
	cfgData, err := clientcmd.Write(*cfg)
	assert.NilError(t, err)
	_, err = kcFile.Write(cfgData)
	assert.NilError(t, err)
	kcFile.Close()

	epDefault, err := FromKubeConfig(kcFile.Name(), "", "")
	assert.NilError(t, err)
	epContext2, err := FromKubeConfig(kcFile.Name(), "context2", "namespace-override")
	assert.NilError(t, err)
	assert.NilError(t, save(store, epDefault, "embed-default-context"))
	assert.NilError(t, save(store, epContext2, "embed-context2"))

	rawNoTLSMeta, err := store.GetMetadata("raw-notls")
	assert.NilError(t, err)
	rawNoTLSSkipMeta, err := store.GetMetadata("raw-notls-skip")
	assert.NilError(t, err)
	rawTLSMeta, err := store.GetMetadata("raw-tls")
	assert.NilError(t, err)
	embededDefaultMeta, err := store.GetMetadata("embed-default-context")
	assert.NilError(t, err)
	embededContext2Meta, err := store.GetMetadata("embed-context2")
	assert.NilError(t, err)

	rawNoTLS := EndpointFromContext(rawNoTLSMeta)
	rawNoTLSSkip := EndpointFromContext(rawNoTLSSkipMeta)
	rawTLS := EndpointFromContext(rawTLSMeta)
	embededDefault := EndpointFromContext(embededDefaultMeta)
	embededContext2 := EndpointFromContext(embededContext2Meta)

	rawNoTLSEP, err := rawNoTLS.WithTLSData(store, "raw-notls")
	assert.NilError(t, err)
	checkClientConfig(t, rawNoTLSEP, "https://test", "test", nil, nil, nil, false)
	rawNoTLSSkipEP, err := rawNoTLSSkip.WithTLSData(store, "raw-notls-skip")
	assert.NilError(t, err)
	checkClientConfig(t, rawNoTLSSkipEP, "https://test", "test", nil, nil, nil, true)
	rawTLSEP, err := rawTLS.WithTLSData(store, "raw-tls")
	assert.NilError(t, err)
	checkClientConfig(t, rawTLSEP, "https://test", "test", []byte("ca"), []byte("cert"), []byte("key"), true)
	embededDefaultEP, err := embededDefault.WithTLSData(store, "embed-default-context")
	assert.NilError(t, err)
	checkClientConfig(t, embededDefaultEP, "https://server1", "namespace1", nil, []byte("cert"), []byte("key"), true)
	embededContext2EP, err := embededContext2.WithTLSData(store, "embed-context2")
	assert.NilError(t, err)
	checkClientConfig(t, embededContext2EP, "https://server2", "namespace-override", []byte("ca"), []byte("cert"), []byte("key"), false)
}

func checkClientConfig(t *testing.T, ep Endpoint, server, namespace string, ca, cert, key []byte, skipTLSVerify bool) {
	config := ep.KubernetesConfig()
	cfg, err := config.ClientConfig()
	assert.NilError(t, err)
	ns, _, _ := config.Namespace()
	assert.Equal(t, server, cfg.Host)
	assert.Equal(t, namespace, ns)
	assert.DeepEqual(t, ca, cfg.CAData)
	assert.DeepEqual(t, cert, cfg.CertData)
	assert.DeepEqual(t, key, cfg.KeyData)
	assert.Equal(t, skipTLSVerify, cfg.Insecure)
}

func save(s store.Writer, ep Endpoint, name string) error {
	meta := store.Metadata{
		Endpoints: map[string]interface{}{
			KubernetesEndpoint: ep.EndpointMeta,
		},
		Name: name,
	}
	if err := s.CreateOrUpdate(meta); err != nil {
		return err
	}
	return s.ResetEndpointTLSMaterial(name, KubernetesEndpoint, ep.TLSData.ToStoreTLSData())
}

func TestSaveLoadGKEConfig(t *testing.T) {
	store := store.New(t.TempDir(), testStoreCfg)
	cfg, err := clientcmd.LoadFromFile("testdata/gke-kubeconfig")
	assert.NilError(t, err)
	clientCfg := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{})
	expectedCfg, err := clientCfg.ClientConfig()
	assert.NilError(t, err)
	ep, err := FromKubeConfig("testdata/gke-kubeconfig", "", "")
	assert.NilError(t, err)
	assert.NilError(t, save(store, ep, "gke-context"))
	persistedMetadata, err := store.GetMetadata("gke-context")
	assert.NilError(t, err)
	persistedEPMeta := EndpointFromContext(persistedMetadata)
	assert.Check(t, persistedEPMeta != nil)
	persistedEP, err := persistedEPMeta.WithTLSData(store, "gke-context")
	assert.NilError(t, err)
	persistedCfg := persistedEP.KubernetesConfig()
	actualCfg, err := persistedCfg.ClientConfig()
	assert.NilError(t, err)
	assert.DeepEqual(t, expectedCfg.AuthProvider, actualCfg.AuthProvider)
}

func TestSaveLoadEKSConfig(t *testing.T) {
	store := store.New(t.TempDir(), testStoreCfg)
	cfg, err := clientcmd.LoadFromFile("testdata/eks-kubeconfig")
	assert.NilError(t, err)
	clientCfg := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{})
	expectedCfg, err := clientCfg.ClientConfig()
	assert.NilError(t, err)
	ep, err := FromKubeConfig("testdata/eks-kubeconfig", "", "")
	assert.NilError(t, err)
	assert.NilError(t, save(store, ep, "eks-context"))
	persistedMetadata, err := store.GetMetadata("eks-context")
	assert.NilError(t, err)
	persistedEPMeta := EndpointFromContext(persistedMetadata)
	assert.Check(t, persistedEPMeta != nil)
	persistedEP, err := persistedEPMeta.WithTLSData(store, "eks-context")
	assert.NilError(t, err)
	persistedCfg := persistedEP.KubernetesConfig()
	actualCfg, err := persistedCfg.ClientConfig()
	assert.NilError(t, err)
	assert.DeepEqual(t, expectedCfg.ExecProvider, actualCfg.ExecProvider)
}

func TestSaveLoadK3SConfig(t *testing.T) {
	store := store.New(t.TempDir(), testStoreCfg)
	cfg, err := clientcmd.LoadFromFile("testdata/k3s-kubeconfig")
	assert.NilError(t, err)
	clientCfg := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{})
	expectedCfg, err := clientCfg.ClientConfig()
	assert.NilError(t, err)
	ep, err := FromKubeConfig("testdata/k3s-kubeconfig", "", "")
	assert.NilError(t, err)
	assert.NilError(t, save(store, ep, "k3s-context"))
	persistedMetadata, err := store.GetMetadata("k3s-context")
	assert.NilError(t, err)
	persistedEPMeta := EndpointFromContext(persistedMetadata)
	assert.Check(t, persistedEPMeta != nil)
	persistedEP, err := persistedEPMeta.WithTLSData(store, "k3s-context")
	assert.NilError(t, err)
	persistedCfg := persistedEP.KubernetesConfig()
	actualCfg, err := persistedCfg.ClientConfig()
	assert.NilError(t, err)
	assert.Check(t, len(actualCfg.Username) > 0)
	assert.Check(t, len(actualCfg.Password) > 0)
	assert.Equal(t, expectedCfg.Username, actualCfg.Username)
	assert.Equal(t, expectedCfg.Password, actualCfg.Password)
}

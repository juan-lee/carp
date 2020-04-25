package remote

import (
	"bytes"
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	client.Client
	factory cmdutil.Factory
}

func NewClient(kubeconfigBytes []byte) (*Client, error) {
	// Build kubeconfig for remote workload cluster
	clientconfig, err := clientcmd.NewClientConfigFromBytes(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote clientconfig: %w", err)
	}

	restClient, err := clientconfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create remote restclient: %w", err)
	}

	kubeclient, err := client.New(restClient, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create remote kubeclient: %w", err)
	}

	getter, err := NewRESTClientGetter(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote restclient getter: %w", err)
	}
	factory := cmdutil.NewFactory(getter)

	return &Client{
		kubeclient,
		factory,
	}, nil
}

/// TODO(ace): figure out how to get apply logic without the factory/cli tools.
// 3-way merge and apply semantics are hard. Server-side apply is harder.
func (c *Client) Apply(url string) (stdout *bytes.Buffer, stderr *bytes.Buffer, err error) {
	return c.do(url, rawFn)

}

// TODO(ace): switch to krusty or shell to kustomize binary. This is reliable
// but lacks newer kustomize features
func (c *Client) Kustomize(url string, mutator ApplyOptionsMutateFn) (stdout *bytes.Buffer, stderr *bytes.Buffer, err error) {
	return c.do(url, kustomizeFn)
}

func (c *Client) do(url string, mutateFn ApplyOptionsMutateFn) (stdout *bytes.Buffer, stderr *bytes.Buffer, err error) {
	stdio := bytes.NewBuffer(nil)
	errio := bytes.NewBuffer(nil)
	streams := genericclioptions.IOStreams{In: stdio, Out: stdio, ErrOut: errio}
	cmd := apply.NewCmdApply("kubectl", c.factory, streams)
	opts := apply.NewApplyOptions(streams)

	// Apply file configuration, either raw or kustomize
	mutateFn(opts, url)

	if err := opts.Complete(c.factory, cmd); err != nil {
		return nil, nil, fmt.Errorf("failed to complete apply options: %w", err)
	}

	return stdio, errio, opts.Run()
}

type ApplyOptionsMutateFn func(opts *apply.ApplyOptions, url string)

var kustomizeFn ApplyOptionsMutateFn = func(opts *apply.ApplyOptions, url string) {
	opts.DeleteFlags.FileNameFlags.Kustomize = &url
}

var rawFn ApplyOptionsMutateFn = func(opts *apply.ApplyOptions, url string) {
	opts.DeleteFlags.FileNameFlags.Filenames = &[]string{url}
}

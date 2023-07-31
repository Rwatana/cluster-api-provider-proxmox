package instance

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	infrav1 "github.com/sp-yduck/cluster-api-provider-proxmox/api/v1beta1"
	"github.com/sp-yduck/cluster-api-provider-proxmox/cloud/cloudinit"
)

const (
	userSnippetPathFormat = "snippets/%s-user.yml"
)

// reconcileCloudInit
func (s *Service) reconcileCloudInit(ctx context.Context, bootstrap string) error {
	// user
	if err := s.reconcileCloudInitUser(ctx, bootstrap); err != nil {
		return err
	}
	return nil
}

// delete CloudConfig
func (s *Service) deleteCloudConfig(ctx context.Context) error {
	storageName := s.scope.GetStorage().Name
	path := userSnippetPath(s.scope.Name())
	volumeID := fmt.Sprintf("%s:%s", storageName, path)

	node, err := s.client.Node(ctx, s.scope.NodeName())
	if err != nil {
		return err
	}
	storage, err := s.client.Storage(ctx, storageName)
	if err != nil {
		return err
	}
	storage.Node = node.Node
	return storage.DeleteVolume(ctx, volumeID)
}

func (s *Service) reconcileCloudInitUser(ctx context.Context, bootstrap string) error {
	vmName := s.scope.Name()
	config := s.scope.GetCloudInit().User

	bootstrapConfig, err := cloudinit.ParseUser(bootstrap)
	if err != nil {
		return err
	}
	base := baseUserData(vmName)
	if config != nil {
		base, err = cloudinit.MergeUsers(*config, *base)
		if err != nil {
			return err
		}
	}
	cloudConfig, err := cloudinit.MergeUsers(*base, *bootstrapConfig)
	if err != nil {
		return err
	}
	configYaml, err := cloudinit.GenerateUserYaml(*cloudConfig)
	if err != nil {
		return err
	}
	klog.Info(configYaml)

	// to do: should be set via API
	vnc, err := s.vncClient(s.scope.NodeName())
	if err != nil {
		return err
	}
	defer vnc.Close()
	filePath := fmt.Sprintf("%s/%s", s.scope.GetStorage().Path, userSnippetPath(vmName))
	if err := vnc.WriteFile(context.TODO(), configYaml, filePath); err != nil {
		return errors.Errorf("failed to write file error : %v", err)
	}

	return nil
}

func userSnippetPath(vmName string) string {
	return fmt.Sprintf(userSnippetPathFormat, vmName)
}

func baseUserData(vmName string) *infrav1.User {
	return &infrav1.User{
		HostName: vmName,
		Packages: []string{"qemu-guest-agent"},
		RunCmd:   []string{"systemctl start qemu-guest-agent"},
	}
}

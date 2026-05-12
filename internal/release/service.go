package release

import "context"

type PortableStatus struct {
	ArtifactName   string   `json:"artifact_name"`
	Version        string   `json:"version"`
	Ready          bool     `json:"ready"`
	IncludedFiles  []string `json:"included_files"`
	PendingActions []string `json:"pending_actions"`
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Status(ctx context.Context) PortableStatus {
	_ = ctx
	return PortableStatus{
		ArtifactName: "tokenbridge.zip",
		Version:      "0.1.0-alpha",
		Ready:        false,
		IncludedFiles: []string{
			"tokenbridge.exe",
			"config.yaml",
			"data/",
			"logs/",
		},
		PendingActions: []string{
			"嵌入前端静态资源",
			"执行真实可执行文件打包",
			"生成 zip 与校验信息",
		},
	}
}

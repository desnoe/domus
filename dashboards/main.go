package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/desnoe/homecode/dashboards/internal/builder"
	"github.com/desnoe/homecode/dashboards/internal/dashboards"
	dashboard "github.com/grafana/grafana-foundation-sdk/go/dashboardv2"
)

type folderManifest struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   folderMetadata `json:"metadata"`
	Spec       folderSpec     `json:"spec"`
}

type folderMetadata struct {
	Name string `json:"name"`
}

type folderSpec struct {
	Title string `json:"title"`
}

func main() {
	envFlag := flag.String("env", "prod", "Target environment: prod, preview, pr")
	branchFlag := flag.String("branch", "", "Branch name for preview environment")
	prFlag := flag.String("pr", "", "PR number for PR environment")
	folderFlag := flag.String("folder", "", "Override folder UID")
	outDirFlag := flag.String("out", "./resources", "Directory where manifest files will be emitted")
	listFlag := flag.Bool("list", false, "List configured dashboards and exit")
	folderInfoFlag := flag.Bool("folder-info", false, "Output target folder UID and exit")
	flag.Parse()

	var slug string
	switch builder.Env(*envFlag) {
	case builder.EnvPreview:
		slug = *branchFlag
		if slug == "" {
			slug = "local"
		}
	case builder.EnvPR:
		slug = *prFlag
		if slug == "" {
			slug = "0"
		}
	default:
		slug = "main"
	}

	cfg := builder.NewConfig(builder.Env(*envFlag), slug, *folderFlag)

	if *folderInfoFlag {
		fmt.Printf("FOLDER_UID=%s\nFOLDER_NAME=%s\n", cfg.FolderUID, cfg.FolderName)
		return
	}

	builders := dashboards.All(cfg)

	if *listFlag {
		fmt.Printf("Configured Dashboards for %s (%s):\n", cfg.Env, cfg.FolderUID)
		for _, b := range builders {
			m, err := b.Build()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error building dashboard: %v\n", err)
				os.Exit(1)
			}
			title := ""
			if d, ok := m.Spec.(dashboard.Dashboard); ok {
				title = d.Title
			}
			fmt.Printf(" - %s (%s)\n", m.Metadata.Name, title)
		}
		return
	}

	// 1. Write Folder manifest
	folderDir := filepath.Join(*outDirFlag, "Folder")
	if err := os.MkdirAll(folderDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create folder directory: %v\n", err)
		os.Exit(1)
	}

	fManifest := folderManifest{
		APIVersion: "folder.grafana.app/v1",
		Kind:       "Folder",
		Metadata:   folderMetadata{Name: cfg.FolderUID},
		Spec:       folderSpec{Title: cfg.FolderName},
	}
	fBytes, err := json.MarshalIndent(fManifest, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal folder manifest: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(folderDir, cfg.FolderUID+".json"), fBytes, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write folder manifest: %v\n", err)
		os.Exit(1)
	}

	// 2. Write Dashboard manifests
	dashDir := filepath.Join(*outDirFlag, "Dashboard")
	if err := os.MkdirAll(dashDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dashboard directory: %v\n", err)
		os.Exit(1)
	}

	for _, b := range builders {
		m, err := b.Build()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to build dashboard manifest: %v\n", err)
			os.Exit(1)
		}

		mBytes, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal dashboard %s: %v\n", m.Metadata.Name, err)
			os.Exit(1)
		}

		manifestPath := filepath.Join(dashDir, m.Metadata.Name+".json")
		if err := os.WriteFile(manifestPath, mBytes, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write manifest %s: %v\n", manifestPath, err)
			os.Exit(1)
		}
	}

	fmt.Printf("Generated %d dashboard manifests and folder %q in %s\n", len(builders), cfg.FolderUID, *outDirFlag)
}

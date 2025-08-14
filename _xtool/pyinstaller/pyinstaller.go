package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/goplus/llgo/_xtool/pyassets"
	"github.com/goplus/llgo/_xtool/pydyn"
)

// Installer 负责安装 Python 包
type Installer struct {
	githubClient *github.Client
	releaseOwner string
	releaseRepo  string
}

// NewInstaller 创建新的安装器实例
func NewInstaller() *Installer {
	return &Installer{
		githubClient: github.NewClient(nil),
		releaseOwner: "Bigdata-shiyang",
		releaseRepo:  "test",
	}
}

// InstallPackage 安装指定的 Python 包
func (i *Installer) InstallPackage(packageName string) error {
	fmt.Printf("Installing Python package: %s\n", packageName)

	// 1. 获取 python-build-standalone 路径
	standalonePath, err := i.getStandalonePath()
	if err != nil {
		return fmt.Errorf("failed to get standalone path: %w", err)
	}

	// 2. 确定 site-packages 路径
	sitePackagesPath := filepath.Join(standalonePath, "python", "lib", "python3.12", "site-packages")

	// 3. 从 GitHub Release 下载 .whl 文件到 site-packages
	whlPath, err := i.downloadWheelFileToSitePackages(packageName, sitePackagesPath)
	if err != nil {
		return fmt.Errorf("failed to download wheel file: %w", err)
	}
	defer os.Remove(whlPath) // 清理临时文件

	// 4. 解压 .whl 文件到 site-packages
	err = i.extractWheelToSitePackages(whlPath, sitePackagesPath, packageName)
	if err != nil {
		return fmt.Errorf("failed to extract wheel: %w", err)
	}

	fmt.Printf("Successfully installed %s to %s\n", packageName, sitePackagesPath)
	return nil
}

// getStandalonePath 获取 python-build-standalone 路径
func (i *Installer) getStandalonePath() (string, error) {
	// // 使用用户提供的 python-build-standalone 路径
	// standalonePath := "/usr/local/opt/python-build-standalone-3.12.11"
	// if _, err := os.Stat(standalonePath); err == nil {
	// 	return standalonePath, nil
	// }

	cwd, _ := os.Getwd()
	pyHome := pydyn.GetPyHome("")
	if pyHome == "" {
		var err error
		// 从二进制内置资产解压一套可用的 Python 到临时目录
		pyHome, err = pyassets.ExtractToDir(cwd)
		if err != nil {
			log.Fatalf("extract embedded python failed: %v\n", err)
		}
	}
	fmt.Printf("pyHome: %s\n", pyHome)
	if err := pydyn.ApplyEnv(pyHome); err != nil {
		log.Fatalf("set py env failed: %v\n", err)
	}

	return pyHome, nil
}

// downloadWheelFileToSitePackages 从 GitHub Release 下载 .whl 文件到 site-packages
func (i *Installer) downloadWheelFileToSitePackages(packageName, sitePackagesPath string) (string, error) {
	fmt.Printf("Downloading wheel file for %s...\n", packageName)

	// 1. 获取最新的 release
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	releases, _, err := i.githubClient.Repositories.ListReleases(ctx, i.releaseOwner, i.releaseRepo, &github.ListOptions{PerPage: 10})
	if err != nil {
		return "", fmt.Errorf("failed to list releases: %w", err)
	}

	if len(releases) == 0 {
		return "", fmt.Errorf("no releases found")
	}

	// 2. 查找包含指定包的 release
	var targetRelease *github.RepositoryRelease
	var targetAsset *github.ReleaseAsset

	for _, release := range releases {
		for _, asset := range release.Assets {
			if strings.Contains(strings.ToLower(*asset.Name), strings.ToLower(packageName)) &&
				strings.HasSuffix(*asset.Name, ".whl") {
				targetRelease = release
				targetAsset = asset
				break
			}
		}
		if targetRelease != nil {
			break
		}
	}

	if targetRelease == nil || targetAsset == nil {
		return "", fmt.Errorf("no wheel file found for package %s", packageName)
	}

	fmt.Printf("Found wheel file: %s in release %s\n", *targetAsset.Name, *targetRelease.TagName)

	// 3. 下载 .whl 文件到 site-packages
	whlPath := filepath.Join(sitePackagesPath, *targetAsset.Name)

	// 下载文件
	resp, err := http.Get(*targetAsset.BrowserDownloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// 创建文件
	outFile, err := os.Create(whlPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	fmt.Printf("Downloaded %s (%d bytes) to %s\n", *targetAsset.Name, *targetAsset.Size, whlPath)
	return whlPath, nil
}

// extractWheelToSitePackages 解压 .whl 文件到 site-packages
func (i *Installer) extractWheelToSitePackages(whlPath, sitePackagesPath, packageName string) error {
	fmt.Printf("Extracting wheel file: %s\n", whlPath)

	// 打开 ZIP 文件
	reader, err := zip.OpenReader(whlPath)
	if err != nil {
		return fmt.Errorf("failed to open wheel file: %w", err)
	}
	defer reader.Close()

	// 解压文件到 site-packages
	for _, file := range reader.File {
		filePath := filepath.Join(sitePackagesPath, file.Name)

		// 创建目录
		if file.FileInfo().IsDir() {
			os.MkdirAll(filePath, file.Mode())
			continue
		}

		// 创建父目录
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// 创建文件
		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		// 打开 ZIP 中的文件
		sourceFile, err := file.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open file in zip: %w", err)
		}

		// 复制内容
		_, err = io.Copy(outFile, sourceFile)
		outFile.Close()
		sourceFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy file content: %w", err)
		}
	}

	fmt.Printf("Extracted wheel to: %s\n", sitePackagesPath)
	return nil
}

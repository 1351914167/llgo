package pyinstaller

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/go-github/v69/github"
	"github.com/goplus/llgo/_xtool/pyassets"
	"github.com/goplus/llgo/_xtool/pydyn"
)

// Installer that installs Python packages
type Installer struct {
	githubClient *github.Client
	releaseOwner string
	releaseRepo  string
}

// NewInstaller creates a new installer instance
func NewInstaller() *Installer {
	return &Installer{
		githubClient: github.NewClient(nil),
		releaseOwner: "Bigdata-shiyang",
		releaseRepo:  "test",
	}
}

// InstallPackage installs the specified Python package
func (i *Installer) InstallPackage(packageName string) error {
	fmt.Printf("Installing Python package: %s\n", packageName)

	// 1. Get python-build-standalone path
	standalonePath, err := i.getStandalonePath()
	if err != nil {
		return fmt.Errorf("failed to get standalone path: %w", err)
	}

	// 2. Determine site-packages path
	sitePackagesPath := filepath.Join(standalonePath, "lib", "python3.12", "site-packages")

	// Skip if already installed
	if isInstalled(packageName, sitePackagesPath) {
		fmt.Printf("%s already installed in %s, skip\n", packageName, sitePackagesPath)
		return nil
	}

	// 3. Install via pip
	err = i.InstallPackageWithPip(packageName, sitePackagesPath)
	if err != nil {
		return fmt.Errorf("failed to install package with pip: %w", err)
	}

	// // 3. Download .whl from GitHub Release to site-packages
	// whlPath, err := i.downloadWheelFileToSitePackages(packageName, sitePackagesPath)
	// if err != nil {
	// 	return fmt.Errorf("failed to download wheel file: %w", err)
	// }
	// defer os.Remove(whlPath) // cleanup temp file

	// // 4. Extract .whl into site-packages
	// err = i.extractWheelToSitePackages(whlPath, sitePackagesPath, packageName)
	// if err != nil {
	// 	return fmt.Errorf("failed to extract wheel: %w", err)
	// }

	fmt.Printf("Successfully installed %s to %s\n", packageName, sitePackagesPath)
	return nil
}

// InstallPackageWithPip installs a package using Python's pip (into current Python environment's site-packages or the specified target directory)
func (i *Installer) InstallPackageWithPip(packageName, sitePackagesPath string) error {
	fmt.Printf("Installing Python package via pip: %s\n", packageName)

	py := getPythonExecName()

	// Ensure pip is available
	cmd := exec.Command(py, "-m", "ensurepip", "--upgrade")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Run() // ignore non-zero ensurepip return (some distributions may have pip preinstalled)

	// Optional: upgrade pip (allow failure without blocking)
	cmd = exec.Command(py, "-m", "pip", "install", "--upgrade", "pip", "--no-cache-dir")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Run()

	// Install target package into specified site-packages
	args := []string{"-m", "pip", "install", "--no-cache-dir", "--upgrade", "--target", sitePackagesPath, packageName}
	cmd = exec.Command(py, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pip install failed: %w", err)
	}

	// fmt.Printf("Successfully installed %s to %s via pip\n", packageName, sitePackagesPath)
	return nil
}

// getStandalonePath gets the python-build-standalone path
func (i *Installer) getStandalonePath() (string, error) {
	// // Use user-provided python-build-standalone path
	// standalonePath := "/usr/local/opt/python-build-standalone-3.12.11"
	// if _, err := os.Stat(standalonePath); err == nil {
	// 	return standalonePath, nil
	// }

	root := os.Getenv("LLGO_ROOT")
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working dir: %w", err)
		}
	}
	pyHome := pydyn.GetPyHome("")
	if pyHome == "" {
		var err error
		// Extract an embedded Python to a temp directory from binary assets
		pyHome, err = pyassets.ExtractToDir(root)
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

// // downloadWheelFileToSitePackages downloads a .whl from GitHub Releases into site-packages
// func (i *Installer) downloadWheelFileToSitePackages(packageName, sitePackagesPath string) (string, error) {
// 	fmt.Printf("Downloading wheel file for %s...\n", packageName)

// 	// 1. Get latest releases
// 	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// 	defer cancel()

// 	releases, _, err := i.githubClient.Repositories.ListReleases(ctx, i.releaseOwner, i.releaseRepo, &github.ListOptions{PerPage: 10})
// 	if err != nil {
// 		return "", fmt.Errorf("failed to list releases: %w", err)
// 	}

// 	if len(releases) == 0 {
// 		return "", fmt.Errorf("no releases found")
// 	}

// 	// 2. Find a release that contains the specified package
// 	var targetRelease *github.RepositoryRelease
// 	var targetAsset *github.ReleaseAsset

// 	for _, release := range releases {
// 		for _, asset := range release.Assets {
// 			if strings.Contains(strings.ToLower(*asset.Name), strings.ToLower(packageName)) &&
// 				strings.HasSuffix(*asset.Name, ".whl") {
// 				targetRelease = release
// 				targetAsset = asset
// 				break
// 			}
// 		}
// 		if targetRelease != nil {
// 			break
// 		}
// 	}

// 	if targetRelease == nil || targetAsset == nil {
// 		return "", fmt.Errorf("no wheel file found for package %s", packageName)
// 	}

// 	fmt.Printf("Found wheel file: %s in release %s\n", *targetAsset.Name, *targetRelease.TagName)

// 	// 3. Download .whl file into site-packages
// 	whlPath := filepath.Join(sitePackagesPath, *targetAsset.Name)

// 	// Download file
// 	resp, err := http.Get(*targetAsset.BrowserDownloadURL)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to download file: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
// 	}

// 	// Create file
// 	outFile, err := os.Create(whlPath)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create file: %w", err)
// 	}
// 	defer outFile.Close()

// 	_, err = io.Copy(outFile, resp.Body)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to save file: %w", err)
// 	}

// 	fmt.Printf("Downloaded %s (%d bytes) to %s\n", *targetAsset.Name, *targetAsset.Size, whlPath)
// 	return whlPath, nil
// }

// // extractWheelToSitePackages extracts the .whl into site-packages
// func (i *Installer) extractWheelToSitePackages(whlPath, sitePackagesPath, packageName string) error {
// 	fmt.Printf("Extracting wheel file: %s\n", whlPath)

// 	// Open ZIP file
// 	reader, err := zip.OpenReader(whlPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to open wheel file: %w", err)
// 	}
// 	defer reader.Close()

// 	// Extract files into site-packages
// 	for _, file := range reader.File {
// 		filePath := filepath.Join(sitePackagesPath, file.Name)

// 		// Create directory
// 		if file.FileInfo().IsDir() {
// 			os.MkdirAll(filePath, file.Mode())
// 			continue
// 		}

// 		// Create parent directories
// 		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
// 			return fmt.Errorf("failed to create directory: %w", err)
// 		}

// 		// Create file
// 		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
// 		if err != nil {
// 			return fmt.Errorf("failed to create file: %w", err)
// 		}

// 		// Open file in ZIP
// 		sourceFile, err := file.Open()
// 		if err != nil {
// 			outFile.Close()
// 			return fmt.Errorf("failed to open file in zip: %w", err)
// 		}

// 		// Copy content
// 		_, err = io.Copy(outFile, sourceFile)
// 		outFile.Close()
// 		sourceFile.Close()
// 		if err != nil {
// 			return fmt.Errorf("failed to copy file content: %w", err)
// 		}
// 	}

// 	fmt.Printf("Extracted wheel to: %s\n", sitePackagesPath)
// 	return nil
// }

func isInstalled(pkg, site string) bool {
	if fi, err := os.Stat(filepath.Join(site, pkg)); err == nil && fi.IsDir() {
		return true
	}
	if matches, _ := filepath.Glob(filepath.Join(site, pkg+"-*.dist-info")); len(matches) > 0 {
		return true
	}
	return false
}

func getPythonExecName() string {
	// Always use "python". PATH has been injected by pydyn.ApplyEnv to point to the embedded Python
	return "python"
}

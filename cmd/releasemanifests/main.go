package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	defaultRepository = "stepanusjanu19/envdoctoragent"
	binaryName        = "envdoctor"
)

type archive struct {
	OS     string
	Arch   string
	Path   string
	Name   string
	SHA256 string
}

func main() {
	dist := flag.String("dist", "dist", "release artifact directory")
	version := flag.String("version", "dev", "release version")
	repository := flag.String("repository", defaultRepository, "GitHub repository in owner/name form")
	tag := flag.String("tag", "", "release tag used in download URLs")
	flag.Parse()

	if strings.TrimSpace(*tag) == "" {
		*tag = defaultTag(*version)
	}

	archives, err := collectArchives(*dist)
	if err != nil {
		fatal(err)
	}
	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s", strings.Trim(*repository, "/"), *tag)

	repo := strings.Trim(*repository, "/")
	if repo == "" {
		repo = defaultRepository
	}

	if err := writeHomebrew(*dist, *version, repo, baseURL, archives); err != nil {
		fatal(err)
	}
	if err := writeScoop(*dist, *version, repo, baseURL, archives); err != nil {
		fatal(err)
	}
	if err := writeWinget(*dist, *version, baseURL, archives); err != nil {
		fatal(err)
	}
	if err := writeChocolatey(*dist, *version, repo, baseURL, archives); err != nil {
		fatal(err)
	}
	if err := writeSummary(*dist, *version, repo, *tag); err != nil {
		fatal(err)
	}
}

func collectArchives(dist string) (map[string]archive, error) {
	patterns := []string{
		filepath.Join(dist, "*.tar.gz"),
		filepath.Join(dist, "*.zip"),
	}
	archives := map[string]archive{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, path := range matches {
			name := filepath.Base(path)
			osName, archName, ok := archiveTarget(name)
			if !ok {
				continue
			}
			sum, err := fileSHA256(path)
			if err != nil {
				return nil, err
			}
			archives[osName+"_"+archName] = archive{
				OS:     osName,
				Arch:   archName,
				Path:   path,
				Name:   name,
				SHA256: sum,
			}
		}
	}
	required := []string{"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64", "windows_amd64", "windows_arm64"}
	for _, key := range required {
		if _, ok := archives[key]; !ok {
			return nil, fmt.Errorf("missing release archive for %s in %s", key, dist)
		}
	}
	return archives, nil
}

func archiveTarget(name string) (string, string, bool) {
	re := regexp.MustCompile(`_(linux|darwin|windows)_(amd64|arm64)\.(tar\.gz|zip)$`)
	matches := re.FindStringSubmatch(name)
	if len(matches) != 4 {
		return "", "", false
	}
	return matches[1], matches[2], true
}

func writeHomebrew(dist, version, repository, baseURL string, archives map[string]archive) error {
	path := filepath.Join(dist, "package-managers", "homebrew", "envdoctor.rb")
	content := fmt.Sprintf(`class Envdoctor < Formula
  desc "Cross-platform development environment diagnostics CLI"
  homepage "https://github.com/%s"
  version "%s"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "%s/%s"
      sha256 "%s"
    else
      url "%s/%s"
      sha256 "%s"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "%s/%s"
      sha256 "%s"
    else
      url "%s/%s"
      sha256 "%s"
    end
  end

  def install
    bin.install "envdoctor"
  end

  test do
    system "#{bin}/envdoctor", "--version"
  end
end
`, repository, version,
		baseURL, archives["darwin_arm64"].Name, archives["darwin_arm64"].SHA256,
		baseURL, archives["darwin_amd64"].Name, archives["darwin_amd64"].SHA256,
		baseURL, archives["linux_arm64"].Name, archives["linux_arm64"].SHA256,
		baseURL, archives["linux_amd64"].Name, archives["linux_amd64"].SHA256,
	)
	return writeFile(path, []byte(content))
}

func writeScoop(dist, version, repository, baseURL string, archives map[string]archive) error {
	path := filepath.Join(dist, "package-managers", "scoop", "envdoctor.json")
	manifest := map[string]interface{}{
		"version":     version,
		"description": "Cross-platform development environment diagnostics CLI",
		"homepage":    "https://github.com/" + repository,
		"license":     "MIT",
		"bin":         "envdoctor.exe",
		"architecture": map[string]interface{}{
			"64bit": map[string]string{
				"url":  baseURL + "/" + archives["windows_amd64"].Name,
				"hash": archives["windows_amd64"].SHA256,
			},
			"arm64": map[string]string{
				"url":  baseURL + "/" + archives["windows_arm64"].Name,
				"hash": archives["windows_arm64"].SHA256,
			},
		},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'))
}

func writeWinget(dist, version, baseURL string, archives map[string]archive) error {
	dir := filepath.Join(dist, "package-managers", "winget", "StepanusJanu19.Envdoctor", version)
	versionManifest := fmt.Sprintf(`PackageIdentifier: StepanusJanu19.Envdoctor
PackageVersion: %s
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.6.0
`, version)
	localeManifest := `PackageIdentifier: StepanusJanu19.Envdoctor
PackageLocale: en-US
Publisher: stepanusjanu19
PackageName: envdoctor
License: MIT
ShortDescription: Cross-platform development environment diagnostics CLI
ManifestType: defaultLocale
ManifestVersion: 1.6.0
`
	installerManifest := fmt.Sprintf(`PackageIdentifier: StepanusJanu19.Envdoctor
PackageVersion: %s
InstallerType: zip
NestedInstallerType: portable
NestedInstallerFiles:
  - RelativeFilePath: envdoctor.exe
    PortableCommandAlias: envdoctor
Installers:
  - Architecture: x64
    InstallerUrl: %s/%s
    InstallerSha256: %s
  - Architecture: arm64
    InstallerUrl: %s/%s
    InstallerSha256: %s
ManifestType: installer
ManifestVersion: 1.6.0
`, version,
		baseURL, archives["windows_amd64"].Name, strings.ToUpper(archives["windows_amd64"].SHA256),
		baseURL, archives["windows_arm64"].Name, strings.ToUpper(archives["windows_arm64"].SHA256),
	)
	if err := writeFile(filepath.Join(dir, "StepanusJanu19.Envdoctor.yaml"), []byte(versionManifest)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "StepanusJanu19.Envdoctor.locale.en-US.yaml"), []byte(localeManifest)); err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "StepanusJanu19.Envdoctor.installer.yaml"), []byte(installerManifest))
}

func writeChocolatey(dist, version, repository, baseURL string, archives map[string]archive) error {
	dir := filepath.Join(dist, "package-managers", "chocolatey", "envdoctor")
	nuspec := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd">
  <metadata>
    <id>envdoctor</id>
    <version>%s</version>
    <title>envdoctor</title>
    <authors>stepanusjanu19</authors>
    <projectUrl>https://github.com/%s</projectUrl>
    <licenseUrl>https://github.com/%s/blob/main/LICENSE</licenseUrl>
    <requireLicenseAcceptance>false</requireLicenseAcceptance>
    <description>Cross-platform development environment diagnostics CLI.</description>
    <summary>Environment diagnostics and setup planning for developers.</summary>
    <tags>envdoctor diagnostics cli development environment</tags>
  </metadata>
</package>
`, chocolateyVersion(version), repository, repository)
	install := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$packageName = 'envdoctor'
$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$url64 = '%s/%s'

$packageArgs = @{
  PackageName    = $packageName
  UnzipLocation  = $toolsDir
  Url64bit       = $url64
  Checksum64     = '%s'
  ChecksumType64 = 'sha256'
}

Install-ChocolateyZipPackage @packageArgs
`, baseURL, archives["windows_amd64"].Name, archives["windows_amd64"].SHA256)
	if err := writeFile(filepath.Join(dir, "envdoctor.nuspec"), []byte(nuspec)); err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "tools", "chocolateyinstall.ps1"), []byte(install))
}

func writeSummary(dist, version, repository, tag string) error {
	content := fmt.Sprintf(`# Package Manager Artifacts

Generated for envdoctor %s.

- Homebrew formula: homebrew/envdoctor.rb
- Scoop manifest: scoop/envdoctor.json
- Winget manifests: winget/StepanusJanu19.Envdoctor/%s/
- Chocolatey package metadata: chocolatey/envdoctor/

These files are generated for manual publishing. External package-manager publishing is intentionally not automated in this phase.

Repository: https://github.com/%s
Release tag: %s
`, version, version, repository, tag)
	return writeFile(filepath.Join(dist, "package-managers", "README.md"), []byte(content))
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func defaultTag(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "dev"
	}
	semver := regexp.MustCompile(`^\d+\.\d+\.\d+([-.].*)?$`)
	if semver.MatchString(version) {
		return "v" + version
	}
	return version
}

func chocolateyVersion(version string) string {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	re := regexp.MustCompile(`^\d+\.\d+\.\d+([-.][0-9A-Za-z.-]+)?$`)
	if re.MatchString(version) {
		return strings.ReplaceAll(version, "-", "-")
	}
	return "0.0.0-" + strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			return r
		}
		return '-'
	}, version)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "release manifest generation failed: %v\n", err)
	os.Exit(1)
}

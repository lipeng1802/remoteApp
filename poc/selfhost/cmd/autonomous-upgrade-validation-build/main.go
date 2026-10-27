// Builds an isolated public TEST bundle for native autonomous install/upgrade.
// The throwaway release key exists only in this process and is never written.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func write(path string, raw []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func read(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > 100<<20 {
		return nil, os.ErrInvalid
	}
	raw := make([]byte, st.Size())
	if _, err = io.ReadFull(f, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func target(target string) (suffix, cgo, arch string, err error) {
	switch target {
	case "darwin":
		return "", "1", runtime.GOARCH, nil
	case "windows":
		return ".exe", "0", "amd64", nil
	default:
		return "", "", "", os.ErrInvalid
	}
}

func validWindowsRoot(root string) bool {
	if len(root) < 4 || len(root) > 1024 || root[0] < 'A' || root[0] > 'Z' || root[1] != ':' || root[2] != '\\' || strings.Contains(root, "/") || strings.HasSuffix(root, "\\") {
		return false
	}
	for _, component := range strings.Split(root[3:], "\\") {
		if component == "" || component == "." || component == ".." || strings.ContainsAny(component, `<>:"|?*`) {
			return false
		}
	}
	return true
}

type releaseLayout struct {
	V1Stage         string `json:"v1_stage"`
	V1Directory     string `json:"v1_directory"`
	V2Stage         string `json:"v2_stage"`
	V2Directory     string `json:"v2_directory"`
	TransactionRoot string `json:"transaction_root"`
}

func copyFile(source, destination string) error {
	raw, err := read(source)
	if err != nil {
		return err
	}
	return write(destination, raw, 0700)
}

func run() error {
	if len(os.Args) != 8 && len(os.Args) != 9 {
		return os.ErrInvalid
	}
	goexe, module, goos, dest := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	journalPath, vaultPath, productPath := os.Args[5], os.Args[6], os.Args[7]
	for _, path := range []string{goexe, module, dest, journalPath, vaultPath, productPath} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return os.ErrInvalid
		}
	}
	suffix, cgo, arch, err := target(goos)
	if err != nil {
		return err
	}
	transactionRoot := filepath.Join(dest, "transaction-install-root")
	if goos == "windows" {
		if len(os.Args) != 9 || !validWindowsRoot(os.Args[8]) {
			return os.ErrInvalid
		}
		transactionRoot = os.Args[8]
	} else if len(os.Args) != 8 {
		return os.ErrInvalid
	}
	journal, err := read(journalPath)
	if err != nil {
		return err
	}
	vault, err := read(vaultPath)
	if err != nil {
		return err
	}
	product, err := read(productPath)
	if err != nil {
		return err
	}
	if err = os.Mkdir(dest, 0700); err != nil {
		return err
	}
	for name, raw := range map[string][]byte{"trust-journal" + suffix: journal, "identity-vault" + suffix: vault, "tls-product" + suffix: product} {
		if err = write(filepath.Join(dest, name), raw, 0700); err != nil {
			return err
		}
	}

	now := time.Now().Truncate(time.Second)
	root, releaseKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	defer func() {
		for i := range releaseKey {
			releaseKey[i] = 0
		}
	}()
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	issuer, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, caPublic, caPrivate)
	if err != nil {
		return err
	}
	claims := trustprofile.Claims{Version: 1, Service: "autonomous-upgrade-test", Epoch: 1, Endpoint: "https://upgrade-validation.example:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(23 * time.Hour).Unix()}
	p1, err := trustprofile.Sign(releaseKey, claims)
	if err != nil {
		return err
	}
	claims.Epoch = 2
	p2, err := trustprofile.Sign(releaseKey, claims)
	if err != nil {
		return err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	journalHash, vaultHash, productHash := sha256.Sum256(journal), sha256.Sum256(vault), sha256.Sum256(product)
	spec := trustinstall.Specification{Version: 1, Channel: "autonomous-upgrade-test-" + hex.EncodeToString(nonce[:]), Root: hex.EncodeToString(root), Service: claims.Service, MinimumEpoch: 1, HelperDigest: hex.EncodeToString(journalHash[:]), Bundled: p1, Test: true}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	compile := func(version string, current trustinstall.Specification) error {
		if err := trustinstall.Compile(ctx, goexe, module, goos, filepath.Join(dest, "trust-install-"+version+suffix), current, now); err != nil {
			return err
		}
		if err := trustinstall.CompileAutonomousEnrollmentInstall(ctx, goexe, module, goos, filepath.Join(dest, "autonomous-enrollment-install-"+version+suffix), current, hex.EncodeToString(vaultHash[:]), now); err != nil {
			return err
		}
		return trustinstall.CompileAutonomousProductClient(ctx, goexe, module, goos, filepath.Join(dest, "installed-autonomous-product-"+version+suffix), current, hex.EncodeToString(vaultHash[:]), hex.EncodeToString(productHash[:]), now)
	}
	if err = compile("v1", spec); err != nil {
		return err
	}
	spec1 := spec
	spec.MinimumEpoch, spec.Bundled = 2, p2
	if err = compile("v2", spec); err != nil {
		return err
	}
	if err = os.Mkdir(filepath.Join(dest, "releases"), 0700); err != nil {
		return err
	}
	if err = os.Mkdir(filepath.Join(dest, "bootstrap"), 0700); err != nil {
		return err
	}
	if err = os.Mkdir(filepath.Join(dest, "bootstrap", "sets"), 0700); err != nil {
		return err
	}
	makeRelease := func(version string, current trustinstall.Specification, epoch uint64) (string, installrelease.Verified, error) {
		var stageNonce [16]byte
		if _, err := rand.Read(stageNonce[:]); err != nil {
			return "", installrelease.Verified{}, err
		}
		stageName := ".staging-" + hex.EncodeToString(stageNonce[:])
		source, err := os.MkdirTemp(filepath.Dir(dest), ".signed-release-source-")
		if err != nil {
			return "", installrelease.Verified{}, err
		}
		defer os.RemoveAll(source)
		fixed := map[string]string{
			"autonomous-enrollment-install" + suffix: "autonomous-enrollment-install-" + version + suffix,
			"identity-vault" + suffix:                "identity-vault" + suffix,
			"installed-autonomous-product" + suffix:  "installed-autonomous-product-" + version + suffix,
			"tls-product" + suffix:                   "tls-product" + suffix,
			"trust-install" + suffix:                 "trust-install-" + version + suffix,
			"trust-journal" + suffix:                 "trust-journal" + suffix,
		}
		for destination, sourceName := range fixed {
			if err := copyFile(filepath.Join(dest, sourceName), filepath.Join(source, destination)); err != nil {
				return "", installrelease.Verified{}, err
			}
		}
		manifest, err := installrelease.BuildManifest(source, current.Channel, goos, epoch)
		if err != nil {
			return "", installrelease.Verified{}, err
		}
		raw, err := installrelease.SignForBuild(releaseKey, manifest)
		if err != nil || write(filepath.Join(source, installrelease.ManifestName), raw, 0600) != nil {
			return "", installrelease.Verified{}, os.ErrInvalid
		}
		transactionDir := filepath.Join(dest, "transaction-"+version)
		payload := filepath.Join(transactionDir, "release-payload")
		if err = os.Mkdir(transactionDir, 0700); err != nil || os.Mkdir(payload, 0700) != nil {
			return "", installrelease.Verified{}, os.ErrInvalid
		}
		payloadFiles, filesErr := installrelease.ExpectedFiles(goos)
		if filesErr != nil {
			return "", installrelease.Verified{}, filesErr
		}
		for _, name := range append([]string{installrelease.ManifestName}, payloadFiles...) {
			if err = copyFile(filepath.Join(source, name), filepath.Join(payload, name)); err != nil {
				return "", installrelease.Verified{}, err
			}
		}
		verified, err := installrelease.ImportStaged(source, dest, stageName, root, current.Channel, goos, current.MinimumEpoch, current.Test)
		return stageName, verified, err
	}
	stage1, verified1, err := makeRelease("v1", spec1, 1)
	if err != nil {
		return err
	}
	stage2, verified2, err := makeRelease("v2", spec, 2)
	if err != nil {
		return err
	}
	name1, err := installrelease.ReleaseDirectoryName(verified1)
	if err != nil {
		return err
	}
	name2, err := installrelease.ReleaseDirectoryName(verified2)
	if err != nil {
		return err
	}
	if err = trustinstall.CompileInstalledReleaseManager(ctx, goexe, module, goos, filepath.Join(dest, "release-manager-v1"+suffix), spec1, stage1, verified1.ManifestDigest, 1, now); err != nil {
		return err
	}
	if err = trustinstall.CompileInstalledReleaseManager(ctx, goexe, module, goos, filepath.Join(dest, "release-manager-v2"+suffix), spec, stage2, verified2.ManifestDigest, 2, now); err != nil {
		return err
	}
	makeTransaction := func(version, operation, stage string, current trustinstall.Specification, verified installrelease.Verified, epoch uint64) error {
		directory := filepath.Join(dest, "transaction-"+version)
		if err := copyFile(filepath.Join(dest, "trust-journal"+suffix), filepath.Join(directory, "trust-journal"+suffix)); err != nil {
			return err
		}
		provisioner := filepath.Join(directory, "install-root-provisioner"+suffix)
		if err := trustinstall.CompileInstallRootProvisioner(ctx, goexe, module, goos, provisioner, transactionRoot, true); err != nil {
			return err
		}
		manager := filepath.Join(directory, "installed-release-manager"+suffix)
		if err := trustinstall.CompileFixedRootInstalledReleaseManager(ctx, goexe, module, goos, manager, current, transactionRoot, stage, verified.ManifestDigest, epoch, now); err != nil {
			return err
		}
		provisionerRaw, provisionerErr := read(provisioner)
		managerRaw, managerErr := read(manager)
		if provisionerErr != nil || managerErr != nil {
			return os.ErrInvalid
		}
		provisionerHash, managerHash := sha256.Sum256(provisionerRaw), sha256.Sum256(managerRaw)
		return trustinstall.CompileInstallTransaction(ctx, goexe, module, goos, filepath.Join(directory, "install-transaction"+suffix), current, transactionRoot, operation, stage, verified.ManifestDigest, epoch, hex.EncodeToString(provisionerHash[:]), hex.EncodeToString(managerHash[:]), now)
	}
	if err = makeTransaction("v1", "install", stage1, spec1, verified1, 1); err != nil {
		return err
	}
	if err = makeTransaction("v2", "upgrade", stage2, spec, verified2, 2); err != nil {
		return err
	}
	makeBootstrap := func(version, operation string, current trustinstall.Specification, epoch uint64) error {
		directory := filepath.Join(dest, "transaction-"+version)
		payload := filepath.Join(directory, "bootstrap-payload")
		if err := os.Mkdir(payload, 0700); err != nil {
			return err
		}
		if err := copyFile(filepath.Join(dest, "trust-journal"+suffix), filepath.Join(payload, "trust-journal"+suffix)); err != nil {
			return err
		}
		if err := trustinstall.CompileFixedRootInstalledReleaseLauncher(ctx, goexe, module, goos, filepath.Join(payload, "installed-release-launcher"+suffix), current, transactionRoot, now); err != nil {
			return err
		}
		manifest, err := installrelease.BuildBootstrapManifest(payload, current.Channel, goos, epoch)
		if err != nil {
			return err
		}
		raw, err := installrelease.SignBootstrapForBuild(releaseKey, manifest)
		if err != nil || write(filepath.Join(payload, installrelease.BootstrapManifestName), raw, 0600) != nil {
			return os.ErrInvalid
		}
		verified, err := installrelease.VerifyBootstrapDirectory(payload, root, current.Channel, goos, current.MinimumEpoch)
		if err != nil {
			return err
		}
		var stageNonce, pointerNonce [16]byte
		if _, err = rand.Read(stageNonce[:]); err != nil {
			return err
		}
		if _, err = rand.Read(pointerNonce[:]); err != nil {
			return err
		}
		return trustinstall.CompileInstalledBootstrapManager(ctx, goexe, module, goos, filepath.Join(directory, "installed-bootstrap-manager"+suffix), current, transactionRoot, operation, ".staging-"+hex.EncodeToString(stageNonce[:]), verified.ManifestDigest, epoch, hex.EncodeToString(pointerNonce[:]), now)
	}
	if err = makeBootstrap("v1", "install", spec1, 1); err != nil {
		return err
	}
	if err = makeBootstrap("v2", "upgrade", spec, 2); err != nil {
		return err
	}
	if err = trustinstall.CompileInstalledBootstrapLauncher(ctx, goexe, module, goos, filepath.Join(dest, "bootstrap-launcher-v1"+suffix), spec1, transactionRoot, now); err != nil {
		return err
	}
	if err = trustinstall.CompileInstalledBootstrapLauncher(ctx, goexe, module, goos, filepath.Join(dest, "bootstrap-launcher-v2"+suffix), spec, transactionRoot, now); err != nil {
		return err
	}
	if err = trustinstall.CompileInstalledReleaseLauncher(ctx, goexe, module, goos, filepath.Join(dest, "release-launcher-v1"+suffix), spec1, now); err != nil {
		return err
	}
	if err = trustinstall.CompileInstalledReleaseLauncher(ctx, goexe, module, goos, filepath.Join(dest, "release-launcher-v2"+suffix), spec, now); err != nil {
		return err
	}
	layout, err := json.Marshal(releaseLayout{V1Stage: stage1, V1Directory: name1, V2Stage: stage2, V2Directory: name2, TransactionRoot: transactionRoot})
	if err != nil || write(filepath.Join(dest, "release-layout.json"), layout, 0600) != nil {
		return os.ErrInvalid
	}
	old, err := json.Marshal(p1)
	if err != nil || write(filepath.Join(dest, "old-profile.json"), old, 0600) != nil {
		return os.ErrInvalid
	}
	validator := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-o", filepath.Join(dest, "autonomous-upgrade-validation"+suffix), "./cmd/autonomous-upgrade-validation")
	validator.Dir = module
	validator.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED="+cgo, "GOOS="+goos, "GOARCH="+arch)
	if validator.Run() != nil {
		return os.ErrInvalid
	}
	hashes := map[string]string{}
	err = filepath.WalkDir(dest, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.Type()&os.ModeSymlink != 0 {
			return os.ErrInvalid
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := read(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		h := sha256.Sum256(raw)
		hashes[filepath.ToSlash(name)] = hex.EncodeToString(h[:])
		return nil
	})
	if err != nil {
		return err
	}
	manifest, err := json.Marshal(hashes)
	if err != nil {
		return err
	}
	return write(filepath.Join(dest, "SHA256.json"), manifest, 0600)
}

func main() {
	if err := run(); err != nil {
		fmt.Println("BUILD_FAILED isolated autonomous upgrade bundle preserved")
		os.Exit(1)
	}
	if _, err := fmt.Println("BUILD_READY isolated autonomous upgrade bundle; no production key or installation used"); err != nil {
		os.Exit(1)
	}
}

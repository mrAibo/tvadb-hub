package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ADBKit/internal/tuning"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "keygen":
		err = runKeygen(os.Args[2:])
	case "sign":
		err = runSign(os.Args[2:])
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "DroidSphere Safe Tuning feed tool")
	fmt.Fprintln(os.Stderr, "  droidsphere-feed keygen --private <private.key> --public <public.key>")
	fmt.Fprintln(os.Stderr, "  droidsphere-feed sign --private <private.key> --payload <payload.json> --out <feed.json>")
}

func runKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	privatePath := fs.String("private", "", "private key output path")
	publicPath := fs.String("public", "", "public key output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*privatePath) == "" || strings.TrimSpace(*publicPath) == "" {
		return errors.New("--private and --public are required")
	}
	if samePath(*privatePath, *publicPath) {
		return errors.New("private and public key paths must be different")
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate Ed25519 key: %w", err)
	}
	if err := writeNewFile(*privatePath, []byte(base64.StdEncoding.EncodeToString(privateKey)+"\n"), 0o600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}
	if err := writeNewFile(*publicPath, []byte(base64.StdEncoding.EncodeToString(publicKey)+"\n"), 0o644); err != nil {
		_ = os.Remove(*privatePath)
		return fmt.Errorf("write public key: %w", err)
	}

	sum := sha256.Sum256(publicKey)
	fmt.Printf("created Ed25519 key pair; key id %s\n", hex.EncodeToString(sum[:8]))
	fmt.Printf("private key: %s (keep secret; do not commit)\n", *privatePath)
	fmt.Printf("public key:  %s\n", *publicPath)
	return nil
}

func runSign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	privatePath := fs.String("private", "", "base64 Ed25519 private key")
	payloadPath := fs.String("payload", "", "unsigned feed payload JSON")
	outputPath := fs.String("out", "", "signed feed envelope JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*privatePath) == "" || strings.TrimSpace(*payloadPath) == "" || strings.TrimSpace(*outputPath) == "" {
		return errors.New("--private, --payload and --out are required")
	}

	privateText, err := os.ReadFile(*privatePath)
	if err != nil {
		return fmt.Errorf("read private key: %w", err)
	}
	privateRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(privateText)))
	if err != nil {
		return fmt.Errorf("private key is not standard base64: %w", err)
	}
	if len(privateRaw) != ed25519.PrivateKeySize {
		return fmt.Errorf("private key must decode to %d bytes", ed25519.PrivateKeySize)
	}

	payload, err := os.ReadFile(*payloadPath)
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}
	envelope, err := tuning.SignFeedPayload(payload, ed25519.PrivateKey(privateRaw))
	if err != nil {
		return err
	}
	if err := writeAtomic(*outputPath, envelope, 0o644); err != nil {
		return fmt.Errorf("write signed feed: %w", err)
	}
	fmt.Printf("signed feed written to %s\n", *outputPath)
	return nil
}

func writeNewFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil && dir != "." {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("output file already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".droidsphere-feed-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && aa == bb
}

package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogs/git-module"
)

func unzip(fname string, dest string) error {
	zr, err := zip.OpenReader(fname)
	if err != nil {
		return err
	}
	defer zr.Close()
	os.MkdirAll(dest, 0777)
	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			os.Mkdir(filepath.Join(dest, file.Name), 0777)
			continue
		}

		fr, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open zipped file %q for reading: %s", file.Name, err.Error())
		}
		if file.Mode()|os.ModeSymlink == file.Mode() {
			// create link
			data, err := ioutil.ReadAll(fr)
			if err != nil {
				return fmt.Errorf("failed to read link target from file %q in zip file: %s", file.Name, err.Error())
			}

			linkdest := filepath.Join(dest, file.Name)
			if err := os.Symlink(string(data), linkdest); err != nil {
				return fmt.Errorf("failed to create symlink %q -> %q: %s", linkdest, string(data), err.Error())
			}
			continue
		}

		fw, err := os.Create(filepath.Join(dest, file.Name))
		if err != nil {
			return fmt.Errorf("failed to create file %q during extraction: %s", file.Name, err.Error())
		}
		_, err = io.Copy(fw, fr)
		if err != nil {
			return fmt.Errorf("failed to extract file %q: %s", file.Name, err.Error())
		}
		fr.Close()
		fw.Close()
	}
	return nil
}

func untar(fname string, dest string) error {
	gzfile, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer gzfile.Close()

	gr, err := gzip.NewReader(gzfile)
	if err != nil {
		return err
	}
	defer gr.Close()

	// TODO: Open through gzip
	tr := tar.NewReader(gr)
	os.MkdirAll(dest, 0777)
	for file, err := tr.Next(); err == nil; file, err = tr.Next() {
		if file.FileInfo().IsDir() {
			os.Mkdir(filepath.Join(dest, file.Name), 0777)
			continue
		}

		if file.FileInfo().Mode()|os.ModeSymlink == file.FileInfo().Mode() {
			// create link
			linkdest := filepath.Join(dest, file.Name)
			if err := os.Symlink(file.Linkname, linkdest); err != nil {
				return fmt.Errorf("failed to create symlink %q -> %q: %s", linkdest, file.Linkname, err.Error())
			}
			continue
		}

		fw, err := os.Create(filepath.Join(dest, file.Name))
		if err != nil {
			return fmt.Errorf("failed to create file %q during extraction: %s", file.Name, err.Error())
		}
		_, err = io.Copy(fw, tr)
		if err != nil {
			return fmt.Errorf("failed to extract file %q: %s", file.Name, err.Error())
		}
		fw.Close()
	}
	return nil
}

func checkfiles(root string) error {
	// expected hashes and link targets for test repository
	hashes := map[string]string{
		"script": "fe8a3874c606877d6731f676b443d2ac",
		"README": "cca1920d0bee2a1d391d50227aefd3f2",
		"deep/nested/directories/with/annex/file/data.dat":     "ef38b7920bff83cd052ae05fc75da404",
		"deep/nested/directories/with/annex/file/unlocked.dat": "520d4ed11f2d101c3e9ea2df9f439b28",
		"unlocked-binary-file":                                 "2bb965fdecf8e2750a5b9fb87a79bf2d",
		"links/data.lnk":                                       "link:../deep/nested/directories/with/annex/file/data.dat",
		"links/readme.lnk":                                     "link:../README",
	}

	filecount := 0

	walkfn := func(curpath string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		filecount++
		relpath, err := filepath.Rel(root, curpath)
		if err != nil {
			return fmt.Errorf("found unexpected path %q outside root %q", curpath, root)
		}

		expHash, ok := hashes[relpath]
		if !ok {
			return fmt.Errorf("unexpected file found: %s", relpath)
		}

		if info.Mode()|os.ModeSymlink == info.Mode() {
			target, err := os.Readlink(curpath)
			if err != nil {
				return fmt.Errorf("failed to read link %q: %s", curpath, err.Error())
			}
			expHash = strings.TrimLeft(expHash, "link:")
			if target != expHash {
				return fmt.Errorf("symlink check failed for %q: expected %q found %q", relpath, expHash, target)
			}
			return nil
		}

		if strings.HasPrefix(expHash, "link:") {
			return fmt.Errorf("expected symlink for %q", relpath)
		}
		fp, err := os.Open(curpath)
		if err != nil {
			return fmt.Errorf("failed to open file %q for reading: %s", curpath, err.Error())
		}
		data, err := ioutil.ReadAll(fp)
		if err != nil {
			return fmt.Errorf("failed reading file %q: %s", curpath, err.Error())
		}
		actsum := md5.Sum(data)
		actualHash := hex.EncodeToString(actsum[:16])
		if expHash != actualHash {
			return fmt.Errorf("hash mismatch for %q: expected %q found %q", relpath, expHash, actualHash)
		}
		return nil
	}

	if err := filepath.Walk(root, walkfn); err != nil {
		return err
	}

	if filecount != len(hashes) {
		return fmt.Errorf("file count mismatch: expected %d found %d", len(hashes), filecount)
	}
	return nil
}

// extractTestRepo extracts the zip archive used for testing.
// Returns the git.Repository.
// Uses external (system) unzip command.
func extractTestRepo() (*git.Repository, error) {
	zipfilepath := "../../testdata/testrepo.zip"

	temprepo, err := ioutil.TempDir("", "libgintestrepo")
	if err != nil {
		return nil, err
	}
	if err := unzip(zipfilepath, temprepo); err != nil {
		return nil, err
	}

	return git.Open(temprepo)
}

func TestZip(t *testing.T) {
	repo, err := extractTestRepo()
	if err != nil {
		t.Fatalf("failed to extract test repository: %s", err.Error())
	}

	defer os.RemoveAll(repo.Path())

	master, err := repo.CatFileCommit("master")
	if err != nil {
		t.Fatalf("failed to get master branch: %s", err.Error())
	}

	zippath, err := ioutil.TempDir("", "libgintestzip")
	if err != nil {
		t.Fatalf("failed creating directory for zip file: %s", err.Error())
	}
	defer os.RemoveAll(zippath)

	zipfile := filepath.Join(zippath, "repo.zip")
	writer := NewZipWriter(repo, master)
	if err := writer.Write(zipfile); err != nil {
		t.Fatalf("error creating zip file: %s", err.Error())
	}

	// unzip and check files
	expath := filepath.Join(zippath, "extracted")
	if err := unzip(zipfile, expath); err != nil {
		t.Fatalf("failed to extract created archive: %s", err.Error())
	}

	defer os.RemoveAll(expath)

	if err := checkfiles(expath); err != nil {
		t.Fatalf("file check failed: %s", err.Error())
	}
}

func TestTar(t *testing.T) {
	repo, err := extractTestRepo()
	if err != nil {
		t.Fatalf("failed to extract test repository: %s", err.Error())
	}

	defer os.RemoveAll(repo.Path())

	master, err := repo.CatFileCommit("master")
	if err != nil {
		t.Fatalf("failed to get master branch: %s", err.Error())
	}

	tarpath, err := ioutil.TempDir("", "libgintesttar")
	if err != nil {
		t.Fatalf("failed creating directory for tar file: %s", err.Error())
	}
	defer os.RemoveAll(tarpath)

	tarfile := filepath.Join(tarpath, "repo.tar.gz")
	writer := NewTarWriter(repo, master)
	if err := writer.Write(tarfile); err != nil {
		t.Fatalf("error creating tar file: %s", err.Error())
	}

	// untar and check files
	expath := filepath.Join(tarpath, "extracted")
	if err := untar(tarfile, expath); err != nil {
		t.Fatalf("failed to extract created archive: %s", err.Error())
	}
	defer os.RemoveAll(expath)

	if err := checkfiles(expath); err != nil {
		t.Fatalf("file check failed: %s", err.Error())
	}
}

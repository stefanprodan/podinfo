package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/go-getter"
	"github.com/spf13/cobra"
)

var (
	codeProjectName string
	codeGitUser     string
	codeVersion     string
	codeProjectPath string
)

var codeCmd = &cobra.Command{
	Use:   `code`,
	Short: "Code commands",
}

var codeInitCmd = &cobra.Command{
	Use:     `init [name]`,
	Short:   "initialize podinfo code repo",
	Example: `  code init demo-app --version=v1.2.0 --git-user=stefanprodan`,
	RunE:    runCodeInit,
}

func init() {
	codeInitCmd.Flags().StringVar(&codeGitUser, "git-user", "", "GitHub user or org")
	codeInitCmd.Flags().StringVar(&codeVersion, "version", "master", "podinfo repo tag or branch name")
	codeInitCmd.Flags().StringVar(&codeProjectPath, "path", ".", "destination repo")

	codeCmd.AddCommand(codeInitCmd)

	rootCmd.AddCommand(codeCmd)
}

func runCodeInit(cmd *cobra.Command, args []string) error {

	if len(codeGitUser) < 0 {
		return fmt.Errorf("--git-user is required")
	}
	if len(args) < 1 {
		return fmt.Errorf("project name is required")
	}

	codeProjectName = args[0]

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting pwd: %s", err)
		os.Exit(1)
	}

	tmpPath := "/tmp/k8s-podinfo"
	versionName := fmt.Sprintf("k8s-podinfo-%s", codeVersion)

	downloadURL := fmt.Sprintf("https://github.com/stefanprodan/podinfo/archive/%s.zip", codeVersion)
	client := &getter.Client{
		Src:  downloadURL,
		Dst:  tmpPath,
		Pwd:  pwd,
		Mode: getter.ClientModeAny,
	}

	fmt.Printf("Downloading %s\n", downloadURL)

	if err := client.Get(); err != nil {
		log.Fatalf("Error downloading: %s", err)
		os.Exit(1)
	}

	pkgFrom := "github.com/stefanprodan/podinfo"
	pkgTo := fmt.Sprintf("github.com/%s/%s", codeGitUser, codeProjectName)

	if err := replaceImports(tmpPath, pkgFrom, pkgTo); err != nil {
		log.Fatalf("Error parsing imports: %s", err)
		os.Exit(1)
	}

	dirs := []string{"pkg", "cmd", "ui", "vendor", ".github"}
	for _, dir := range dirs {

		err = os.MkdirAll(path.Join(codeProjectPath, dir), os.ModePerm)
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}
		if err := copyDir(path.Join(tmpPath, versionName, dir), path.Join(codeProjectPath, dir)); err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}
	}

	files := []string{"Gopkg.toml", "Gopkg.lock"}
	for _, file := range files {
		if err := copyFile(path.Join(tmpPath, versionName, file), path.Join(codeProjectPath, file)); err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}

		fileContent, err := ioutil.ReadFile(path.Join(codeProjectPath, file))
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}

		newContent := strings.Replace(string(fileContent), pkgFrom, pkgTo, -1)
		err = ioutil.WriteFile(path.Join(codeProjectPath, file), []byte(newContent), os.ModePerm)
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}
	}

	projFrom := "stefanprodan/podinfo"
	projTo := fmt.Sprintf("%s/%s", codeGitUser, codeProjectName)

	makeFiles := []string{"Makefile.gh", "Dockerfile.gh"}
	for _, file := range makeFiles {
		fileContent, err := ioutil.ReadFile(path.Join(tmpPath, versionName, file))
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}

		destFile := strings.Replace(file, ".gh", "", -1)
		newContent := strings.Replace(string(fileContent), projFrom, projTo, -1)
		err = ioutil.WriteFile(path.Join(codeProjectPath, destFile), []byte(newContent), os.ModePerm)
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}
	}

	workflows := []string{".github/main.workflow"}
	for _, file := range workflows {
		fileContent, err := ioutil.ReadFile(path.Join(codeProjectPath, file))
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}

		newContent := strings.Replace(string(fileContent), "Dockerfile.gh", "Dockerfile", -1)
		err = ioutil.WriteFile(path.Join(codeProjectPath, file), []byte(newContent), os.ModePerm)
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}
	}

	dockerFiles := []string{"Dockerfile.ci"}
	for _, file := range dockerFiles {
		fileContent, err := ioutil.ReadFile(path.Join(tmpPath, versionName, file))
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}

		newContent := strings.Replace(string(fileContent), projFrom, projTo, -1)
		err = ioutil.WriteFile(path.Join(codeProjectPath, file), []byte(newContent), os.ModePerm)
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}
	}

	travisFiles := []string{"travis.lite.yml"}
	for _, file := range travisFiles {
		fileContent, err := ioutil.ReadFile(path.Join(tmpPath, versionName, file))
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}

		destFile := strings.Replace(file, "travis.lite.yml", ".travis.yml", -1)
		newContent := strings.Replace(string(fileContent), projFrom, projTo, -1)
		err = ioutil.WriteFile(path.Join(codeProjectPath, destFile), []byte(newContent), os.ModePerm)
		if err != nil {
			log.Fatalf("Error: %s", err)
			os.Exit(1)
		}
	}

	err = gitPush()
	if err != nil {
		log.Fatalf("git push error: %s", err)
		os.Exit(1)
	}

	fmt.Println("Initialization finished")
	return nil
}

func gitPush() error {
	cmdPush := fmt.Sprintf("git add . && git commit -m \"sync %s\" && git push", codeVersion)
	cmd := exec.Command("sh", "-c", cmdPush)
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	fmt.Println(string(output))
	return nil
}

func replaceImports(projectPath string, pkgFrom string, pkgTo string) error {
	regexImport, err := regexp.Compile(`(?s)(import(.*?)\)|import.*$)`)
	if err != nil {
		return err
	}

	regexImportedPackage, err := regexp.Compile(`"(.*?)"`)
	if err != nil {
		return err
	}

	found := []string{}

	err = filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".go" {
			bts, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			content := string(bts)
			matches := regexImport.FindAllString(content, -1)
			isExists := false

		isReplaceable:
			for _, each := range matches {
				for _, eachLine := range strings.Split(each, "\n") {
					matchesInline := regexImportedPackage.FindAllString(eachLine, -1)
					if err != nil {
						return err
					}

					for _, eachSubline := range matchesInline {
						if strings.Contains(eachSubline, pkgFrom) {
							isExists = true
							break isReplaceable
						}
					}
				}
			}

			if isExists {
				content = strings.Replace(content, `"`+pkgFrom+`"`, `"`+pkgTo+`"`, -1)
				content = strings.Replace(content, `"`+pkgFrom+`/`, `"`+pkgTo+`/`, -1)
				found = append(found, path)
			}

			err = ioutil.WriteFile(path, []byte(content), info.Mode())
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		fmt.Println("ERROR", err.Error())
	}

	if len(found) == 0 {
		fmt.Println("Nothing replaced")
	} else {
		fmt.Printf("Go imports total %d file replaced\n", len(found))
	}

	return nil
}

func copyDir(src string, dst string) error {
	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = copyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}

			err = copyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return
	}

	err = out.Sync()
	if err != nil {
		return
	}

	si, err := os.Stat(src)
	if err != nil {
		return
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return
	}

	return
}

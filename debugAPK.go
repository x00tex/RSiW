package main

/*

Author: @p00rduck
Date: 2023-05-29
Description: Golang implementation of "debugAPKv1.sh" script.

Usage: go run debugAPK.go [APK_FILE]

*/

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println("Usage: go run main.go [APK_FILE]")
		return
	}

	apk := os.Args[1]
	apktool := "apktool"
	apktoolArgs := []string{}
	debugFlag := false
	if len(os.Args) >= 3 && os.Args[2] == "debug" {
		debugFlag = true
	}
	requiredVersion := "2.5.0"
	installedVersion, err := getInstalledVersion(apktool)
	if err != nil {
		log.Fatal("Failed to check installed apktool version: ", err)
	}

	if installedVersion != requiredVersion {
		apktoolJar := filepath.Join(".", "apktool_2.5.0.jar")
		if _, err := os.Stat(apktoolJar); err == nil {
			fmt.Println("Found apktool_2.5.0.jar file in the current directory. Proceeding...")
			apktool = "java"
			apktoolArgs = append(apktoolArgs, "-jar", apktoolJar)
		} else {
			fmt.Printf("I require apktool version %s but found version %s. Aborting.\n", requiredVersion, installedVersion)
			return
		}
	}

	if _, err := exec.LookPath("keytool"); err != nil {
		log.Fatal("I require keytool but it's not installed. Aborting.")
	}

	if _, err := exec.LookPath("jarsigner"); err != nil {
		log.Fatal("I require jarsigner but it's not installed. Aborting.")
	}

	tmpDir, err := ioutil.TempDir("", "apkdebug")
	if err != nil {
		log.Fatal("Failed to create temporary directory:", err)
	}
	defer os.RemoveAll(tmpDir)

	debugAPK := strings.TrimSuffix(apk, filepath.Ext(apk)) + ".debug.apk"

	if _, err := os.Stat(apk); err == nil {
		fmt.Println("=> Unpacking APK...")
		unpackArgs := append(apktoolArgs, "-q", "d", apk, "-o", filepath.Join(tmpDir, "app"))
		cmd := exec.Command(apktool, unpackArgs...)
		err = processCMD(cmd, debugFlag)
		if err != nil {
			log.Fatal("Failed to unpack APK: ", err)
		}

		fmt.Println("=> Adding debug flag...")
		manifestPath := filepath.Join(tmpDir, "app", "AndroidManifest.xml")
		if err := addDebuggableFlag(manifestPath); err != nil {
			log.Fatal("Failed to add debug flag: ", err)
		}

		fmt.Println("=> Repacking APK...")
		repackArgs := append(apktoolArgs, "-q", "b", filepath.Join(tmpDir, "app"), "--use-aapt2", "-o", debugAPK)
		cmd = exec.Command(apktool, repackArgs...)
		err = processCMD(cmd, debugFlag)
		if err != nil {
			log.Fatal("Failed to repackage APK:", err)
		}

		fmt.Println("=> Signing APK...")
		keyStorePath := filepath.Join(tmpDir, "keystore")
		if err := generateKeyStore(keyStorePath, debugFlag); err != nil {
			log.Fatal("Failed to generate keystore: ", err)
		}

		cmd = exec.Command("jarsigner", "-keystore", keyStorePath, "-storepass", "password", "-keypass", "password", debugAPK, "alias1")
		err = processCMD(cmd, debugFlag)
		if err != nil {
			log.Fatal("Failed to sign APK: ", err)
		}

		fmt.Println("=> Checking your debug APK...")
		if err := verifyAPK(debugAPK); err != nil {
			log.Fatal("Failed to verify debug APK: ", err)
		}

		fmt.Println("\n======")
		fmt.Println("Success!")
		fmt.Println("======")
		fmt.Println("(deleting temporary directory...)")
		fmt.Println("Your debug APK: ", debugAPK)
	} else {
		fmt.Println("File not found: ", apk)
	}
}

func processCMD(cmd *exec.Cmd, debugFlag bool) error {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if debugFlag {
			fmt.Println("Command output:\n", stdout.String())
			fmt.Println("Command error:\n", stderr.String())
		}
		return err
	}
	return nil
}

func getInstalledVersion(apktool string) (string, error) {
	cmd := exec.Command(apktool, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	scanner.Scan()
	version := strings.Fields(scanner.Text())[0]
	return version, scanner.Err()
}

func addDebuggableFlag(manifestPath string) error {
	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return err
	}

	content := string(data)
	content = strings.ReplaceAll(content, "android:debuggable=\"[^\"]*\" *", "")
	content = strings.ReplaceAll(content, "<application ", "<application android:debuggable=\"true\" ")
	if err := ioutil.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		return err
	}

	return nil
}

func generateKeyStore(keyStorePath string, debugFlag bool) error {
	cmd := exec.Command("keytool", "-genkey", "-noprompt",
		"-alias", "alias1",
		"-dname", "CN=Unknown, OU=Unknown, O=Unknown, L=Unknown, S=Unknown, C=Unknown",
		"-keystore", keyStorePath,
		"-keyalg", "RSA",
		"-storepass", "password",
		"-keypass", "password",
	)
	err := processCMD(cmd, debugFlag)
	if err != nil {
		return err
	}

	return nil
}

func verifyAPK(apk string) error {
	cmd := exec.Command("jarsigner", "-verify", apk)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		return err
	}

	output := strings.Split(stdout.String(), "\n")
	for i, line := range output {
		if i >= 2 {
			break
		}
		fmt.Println(line)
	}

	return nil
}

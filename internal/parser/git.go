// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package parser

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// collectGitInfo собирает информацию о Git репозитории.
func collectGitInfo(project *Project) error {
	repoRoot, err := findGitRoot(project.ContractsDir)
	if err != nil {
		return nil
	}

	gitInfo := &GitInfo{}

	if commit, err := getGitCommit(repoRoot); err == nil {
		gitInfo.Commit = commit
	}

	if branch, err := getGitBranch(repoRoot); err == nil {
		gitInfo.Branch = branch
	}

	if tag, err := getGitTag(repoRoot); err == nil && tag != "" {
		gitInfo.Tag = tag
	}

	if dirty, err := isGitDirty(repoRoot); err == nil {
		gitInfo.Dirty = dirty
	}

	if user, err := getGitUserName(repoRoot); err == nil && user != "" {
		gitInfo.User = user
	}

	if email, err := getGitUserEmail(repoRoot); err == nil && email != "" {
		gitInfo.Email = email
	}

	if remoteURL, err := getGitRemoteURL(repoRoot); err == nil && remoteURL != "" {
		gitInfo.RemoteURL = remoteURL
	}

	project.Git = gitInfo
	return nil
}

// findGitRoot находит корень git репозитория.
func findGitRoot(startDir string) (string, error) {
	dir := startDir
	for {
		gitDir := filepath.Join(dir, ".git")
		cmd := exec.Command("test", "-d", gitDir)
		if err := cmd.Run(); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", exec.ErrNotFound
		}
		dir = parent
	}
}

// getGitCommit получает хеш текущего коммита.
func getGitCommit(repoRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getGitBranch получает имя текущей ветки.
func getGitBranch(repoRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getGitTag получает тег текущего коммита.
func getGitTag(repoRoot string) (string, error) {
	cmd := exec.Command("git", "describe", "--exact-match", "--tags", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(output)), nil
}

// isGitDirty проверяет, есть ли незакоммиченные изменения.
func isGitDirty(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "diff", "--quiet")
	cmd.Dir = repoRoot
	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// getGitUserName получает имя пользователя Git.
func getGitUserName(repoRoot string) (string, error) {
	cmd := exec.Command("git", "config", "user.name")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getGitUserEmail получает email пользователя Git.
func getGitUserEmail(repoRoot string) (string, error) {
	cmd := exec.Command("git", "config", "user.email")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getGitRemoteURL получает URL удаленного репозитория.
func getGitRemoteURL(repoRoot string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

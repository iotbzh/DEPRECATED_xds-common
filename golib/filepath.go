package common

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Exists returns whether the given file or directory exists or not
func Exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

// IsDir returns true when the given path is a directory
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}

	return fi.Mode().IsDir()
}

// ResolveEnvVar Resolved environment variable regarding the syntax ${MYVAR}
// or $MYVAR following by a slash or a backslash
func ResolveEnvVar(s string) (string, error) {
	if s == "" {
		return s, nil
	}

	// Resolved tilde : ~/
	if len(s) > 2 && s[:2] == "~/" {
		if usr, err := user.Current(); err == nil {
			s = filepath.Join(usr.HomeDir, s[2:])
		}
	}

	// Resolved ${MYVAR}
	re := regexp.MustCompile("\\${([^}]+)}")
	vars := re.FindAllStringSubmatch(s, -1)
	res := s
	for _, v := range vars {
		val := ""
		if v[1] == "EXEPATH" {
			// Specific case to resolve $EXEPATH or ${EXEPATH} used as current executable path
			exePath := os.Args[0]
			ee, _ := os.Executable()
			exeAbsPath, err := filepath.Abs(ee)
			if err == nil {
				exePath, err = filepath.EvalSymlinks(exeAbsPath)
				if err == nil {
					exePath = filepath.Dir(ee)
				} else {
					exePath = filepath.Dir(exeAbsPath)
				}
			}
			val = exePath

		} else {
			// Get env var value
			val = os.Getenv(v[1])
			if val == "" {
				// Specific case to resolved $HOME or ${HOME} on Windows host
				if runtime.GOOS == "windows" && v[1] == "HOME" {
					if usr, err := user.Current(); err == nil {
						val = usr.HomeDir
					}
				} else {
					return res, fmt.Errorf("ERROR: %s env variable not defined", v[1])
				}
			}
		}

		rer := regexp.MustCompile("\\${" + v[1] + "}")
		res = rer.ReplaceAllString(res, val)
	}

	// Resolved $MYVAR following by a slash (or a backslash for Windows)
	// TODO
	//re := regexp.MustCompile("\\$([^\\/])+/")

	return path.Clean(res), nil
}

// PathNormalize normalizes a linux or windows like path
func PathNormalize(p string) string {
	sep := string(filepath.Separator)
	if sep != "/" {
		return p
	}
	// Replace drive like C: by C/
	res := p
	if p[1:2] == ":" {
		res = p[0:1] + sep + p[2:]
	}
	res = strings.Replace(res, "\\", "/", -1)
	return filepath.Clean(res)
}

// GetUserHome returns the user's home directory or empty string on error
func GetUserHome() string {
	if usr, err := user.Current(); err == nil && usr != nil && usr.HomeDir != "" {
		return usr.HomeDir
	}
	for _, p := range []string{"HOME", "HomePath"} {
		if h := os.Getenv(p); h != "" {
			return h
		}
	}

	return ""
}

package cmd

import (
    "archive/tar"
    "archive/zip"
    "compress/gzip"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "gopkg.in/yaml.v3"

)

// PluginMeta represents metadata defined in plugin.yaml.
type PluginMeta struct {
    Name        string `json:"name" yaml:"name"`
    Description string `json:"description" yaml:"description"`
    Version     string `json:"version" yaml:"version"`
}

// installedPluginsFile stores the list of installed plugin names.
const installedPluginsFile = "installed_plugins.json"

// getInstalledPlugins reads the installed plugins file or returns empty slice.
func getInstalledPlugins() ([]string, error) {
    data, err := os.ReadFile(installedPluginsFile)
    if err != nil {
        if os.IsNotExist(err) {
            return []string{}, nil
        }
        return nil, err
    }
    var list []string
    if err := json.Unmarshal(data, &list); err != nil {
        return nil, err
    }
    return list, nil
}

// saveInstalledPlugins writes the list of installed plugins.
func saveInstalledPlugins(list []string) error {
    data, err := json.Marshal(list)
    if err != nil {
        return err
    }
    return os.WriteFile(installedPluginsFile, data, 0644)
}

// pluginsHandler returns JSON list of available plugins with install status.
func pluginsHandler(w http.ResponseWriter, r *http.Request) {
    pluginsDir := "examples/plugins"
    entries, err := os.ReadDir(pluginsDir)
    if err != nil {
        http.Error(w, "cannot read plugins directory", http.StatusInternalServerError)
        return
    }
    installed, _ := getInstalledPlugins()
    installedSet := make(map[string]struct{})
    for _, name := range installed {
        installedSet[name] = struct{}{}
    }
    var result []map[string]interface{}
    for _, e := range entries {
        if !e.IsDir() {
            continue
        }
        metaPath := filepath.Join(pluginsDir, e.Name(), "plugin.yaml")
        metaData, err := os.ReadFile(metaPath)
        if err != nil {
            // skip plugins without manifest
            continue
        }
        var meta PluginMeta
        if yamlErr := yaml.Unmarshal(metaData, &meta); yamlErr != nil {
            // skip if cannot parse
            continue
        }
        installedFlag := false
        if _, ok := installedSet[meta.Name]; ok {
            installedFlag = true
        }
        result = append(result, map[string]interface{}{
            "name":        meta.Name,
            "description": meta.Description,
            "version":     meta.Version,
            "installed":   installedFlag,
        })
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}

// uploadHandler accepts a zip/tar archive, validates, and extracts to plugins directory.
func uploadHandler(w http.ResponseWriter, r *http.Request) {
    // limit request size to 10MB
    r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
    if err := r.ParseMultipartForm(10 << 20); err != nil {
        http.Error(w, "failed to parse multipart form", http.StatusBadRequest)
        return
    }
    file, header, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "file not provided", http.StatusBadRequest)
        return
    }
    defer file.Close()

    // Determine file type by extension
    filename := header.Filename
    lower := strings.ToLower(filename)
    var tempDir string
    tempDir, err = os.MkdirTemp("", "plugin_upload_*")
    if err != nil {
        http.Error(w, "cannot create temp dir", http.StatusInternalServerError)
        return
    }
    defer os.RemoveAll(tempDir)

    // Save uploaded file to temp
    tmpFilePath := filepath.Join(tempDir, filename)
    tmpFile, err := os.Create(tmpFilePath)
    if err != nil {
        http.Error(w, "cannot create temp file", http.StatusInternalServerError)
        return
    }
    if _, err = io.Copy(tmpFile, file); err != nil {
        tmpFile.Close()
        http.Error(w, "failed to copy file", http.StatusInternalServerError)
        return
    }
    tmpFile.Close()

    // Extract based on format
    if strings.HasSuffix(lower, ".zip") {
        if err = unzip(tmpFilePath, tempDir); err != nil {
            http.Error(w, "failed to unzip: "+err.Error(), http.StatusBadRequest)
            return
        }
    } else if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
        if err = untarGz(tmpFilePath, tempDir); err != nil {
            http.Error(w, "failed to untar gz: "+err.Error(), http.StatusBadRequest)
            return
        }
    } else if strings.HasSuffix(lower, ".tar") {
        if err = untar(tmpFilePath, tempDir); err != nil {
            http.Error(w, "failed to untar: "+err.Error(), http.StatusBadRequest)
            return
        }
    } else {
        http.Error(w, "unsupported archive format", http.StatusBadRequest)
        return
    }

    // Expect a single top‑level directory containing plugin.yaml
    entries, err := os.ReadDir(tempDir)
    if err != nil || len(entries) == 0 {
        http.Error(w, "invalid archive content", http.StatusBadRequest)
        return
    }
    var pluginRoot string
    // If there is exactly one directory, use it; otherwise assume files are at root.
    if len(entries) == 1 && entries[0].IsDir() {
        pluginRoot = filepath.Join(tempDir, entries[0].Name())
    } else {
        pluginRoot = tempDir
    }
    // Validate presence of plugin.yaml
    if _, err = os.Stat(filepath.Join(pluginRoot, "plugin.yaml")); err != nil {
        http.Error(w, "plugin.yaml not found in archive", http.StatusBadRequest)
        return
    }
    // Destination directory (preserve original folder name if available)
    destName := filepath.Base(pluginRoot)
    destPath := filepath.Join("examples/plugins", destName)
    // Remove existing if any
    os.RemoveAll(destPath)
    if err = os.Rename(pluginRoot, destPath); err != nil {
        http.Error(w, "failed to move plugin: "+err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusCreated)
    fmt.Fprint(w, "plugin uploaded successfully")
}

// installHandler marks a plugin as installed.
func installHandler(w http.ResponseWriter, r *http.Request) {
    var payload struct {
        Name string `json:"name"`
    }
    if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
        http.Error(w, "invalid json payload", http.StatusBadRequest)
        return
    }
    if payload.Name == "" {
        http.Error(w, "plugin name required", http.StatusBadRequest)
        return
    }
    installed, err := getInstalledPlugins()
    if err != nil {
        http.Error(w, "cannot read installed plugins", http.StatusInternalServerError)
        return
    }
    // avoid duplicates
    for _, n := range installed {
        if n == payload.Name {
            w.WriteHeader(http.StatusOK)
            fmt.Fprint(w, "already installed")
            return
        }
    }
    installed = append(installed, payload.Name)
    if err = saveInstalledPlugins(installed); err != nil {
        http.Error(w, "cannot save installed plugins", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
    fmt.Fprint(w, "plugin installed")
}

// --- archive helpers ---

func unzip(src, dest string) error {
    r, err := zip.OpenReader(src)
    if err != nil {
        return err
    }
    defer r.Close()
    for _, f := range r.File {
        fpath := filepath.Join(dest, f.Name)
        if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
            return fmt.Errorf("illegal file path: %s", fpath)
        }
        if f.FileInfo().IsDir() {
            os.MkdirAll(fpath, os.ModePerm)
            continue
        }
        if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
            return err
        }
        outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
        if err != nil {
            return err
        }
        rc, err := f.Open()
        if err != nil {
            outFile.Close()
            return err
        }
        _, err = io.Copy(outFile, rc)
        outFile.Close()
        rc.Close()
        if err != nil {
            return err
        }
    }
    return nil
}

func untar(src, dest string) error {
    file, err := os.Open(src)
    if err != nil {
        return err
    }
    defer file.Close()
    tr := tar.NewReader(file)
    return extractTar(tr, dest)
}

func untarGz(src, dest string) error {
    file, err := os.Open(src)
    if err != nil {
        return err
    }
    defer file.Close()
    gz, err := gzip.NewReader(file)
    if err != nil {
        return err
    }
    defer gz.Close()
    tr := tar.NewReader(gz)
    return extractTar(tr, dest)
}

func extractTar(tr *tar.Reader, dest string) error {
    for {
        hdr, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
        target := filepath.Join(dest, hdr.Name)
        switch hdr.Typeflag {
        case tar.TypeDir:
            if err = os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
                return err
            }
        case tar.TypeReg:
            if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
                return err
            }
            outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
            if err != nil {
                return err
            }
            if _, err = io.Copy(outFile, tr); err != nil {
                outFile.Close()
                return err
            }
            outFile.Close()
        }
    }
    return nil
}

func init() {
    // register HTTP routes when dashboard is started
    http.HandleFunc("/plugins", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
            pluginsHandler(w, r)
        case http.MethodPost:
            // Distinguish upload vs install by query param
            if r.URL.Query().Get("action") == "install" {
                installHandler(w, r)
            } else {
                uploadHandler(w, r)
            }
        default:
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        }
    })
}

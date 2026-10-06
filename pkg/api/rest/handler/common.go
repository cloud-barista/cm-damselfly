package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cloudmodel "github.com/cloud-barista/cm-beetle/imdl/cloud-model"
	onpremisemodel "github.com/cloud-barista/cm-beetle/imdl/on-premise-model"
	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
	"github.com/rs/zerolog/log"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func generateRandomString(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	// Encode to base64 and trim to the desired length
	return base64.URLEncoding.EncodeToString(b)[:n], nil
}

func generateUnique15DigitInt() (int, error) {
	// The maximum value for a 15-digit number
	max := new(big.Int)
	max.SetString("999999999999999", 10) // 15-digit maximum value

	// Get *big.Int type of num.
	bigNum, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}

	result, err := bigIntToInt(bigNum)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Converted int:", result)
	}

	return result, nil
}

func bigIntToInt(b *big.Int) (int, error) {
	// Convert big.Int to int64
	if !b.IsInt64() {
		return 0, fmt.Errorf("value out of int64 range")
	}
	// Ensure it's within int range
	i := b.Int64()
	if i > int64(^uint(0)>>1) || i < int64(^int(0)) {
		return 0, fmt.Errorf("value out of int range")
	}
	return int(i), nil
}

func generateUnique15DigitString() (int, error) {
	// The maximum value for a 15-digit number
	max := new(big.Int)
	max.SetString("999999999999999", 10) // 15-digit maximum value

	// Get *big.Int type of num.
	bigNum, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}

	result, err := bigIntToInt(bigNum)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Converted int:", result)
	}

	return result, nil
}

func getSeoulCurrentTime() (string, error) {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		log.Error().Msgf("Failed to Get the Time Value of the Location : [%v]", err)
		return "", err
	}

	currentTime := time.Now().In(loc)
	return currentTime.Format("2006-01-02 15:04:05"), nil
}

func isRunningInContainer() bool {
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		// log.Debug().Msgf("iface.Name: [%v]", iface.Name)
		if strings.HasPrefix(iface.Name, "docker") {
			return true
		}
	}
	return false

	// file, err := os.Open("/proc/1/cgroup")
	// if err != nil {
	//     return false
	// }
	// defer file.Close()

	// scanner := bufio.NewScanner(file)
	// for scanner.Scan() {
	//     if strings.Contains(scanner.Text(), "docker") || strings.Contains(scanner.Text(), "kubepods") {
	//         return true
	//     }
	// }
	// return false
}

// modelTagSource describes where the tagged versions of a migration model module live.
// Sub-module tags in the Git repository are prefixed with the module directory (e.g. 'imdl/v0.1.15').
type modelTagSource struct {
	Owner      string
	Repo       string
	TagPrefix  string
	ModulePath string
}

var (
	infraModelTagSource = modelTagSource{
		Owner:      "cloud-barista",
		Repo:       "cm-beetle",
		TagPrefix:  "imdl/",
		ModulePath: "github.com/cloud-barista/cm-beetle/imdl",
	}
	softwareModelTagSource = modelTagSource{
		Owner:      "cloud-barista",
		Repo:       "cm-grasshopper",
		TagPrefix:  "smdl/",
		ModulePath: "github.com/cloud-barista/cm-grasshopper/smdl",
	}
)

const modelVersionCacheTTL = 10 * time.Minute

type modelVersionCacheEntry struct {
	versions  []string
	fetchedAt time.Time
}

var (
	modelVersionHTTPClient = &http.Client{Timeout: 10 * time.Second}
	modelVersionCacheMu    sync.Mutex
	modelVersionCache      = map[string]modelVersionCacheEntry{}
	releaseVersionPattern  = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
)

// getInfraModelVersions returns the tagged versions of cm-beetle/imdl (on-premise and cloud models) in ascending order.
func getInfraModelVersions() ([]string, error) {
	return getModelTagVersions(infraModelTagSource)
}

// getSoftwareModelVersions returns the tagged versions of cm-grasshopper/smdl (software models) in ascending order.
func getSoftwareModelVersions() ([]string, error) {
	return getModelTagVersions(softwareModelTagSource)
}

// getLatestInfraModelVersion returns the latest tagged version of cm-beetle/imdl.
func getLatestInfraModelVersion() (string, error) {
	versions, err := getInfraModelVersions()
	if err != nil {
		return "", err
	}
	return selectModelVersion("", versions)
}

// getLatestSoftwareModelVersion returns the latest tagged version of cm-grasshopper/smdl.
func getLatestSoftwareModelVersion() (string, error) {
	versions, err := getSoftwareModelVersions()
	if err != nil {
		return "", err
	}
	return selectModelVersion("", versions)
}

// getModelTagVersions gets the tagged versions from the GitHub repository and falls back to the Go module proxy
// (which mirrors the same tags) when GitHub is unavailable (e.g. API rate limit). Results are cached briefly.
func getModelTagVersions(src modelTagSource) ([]string, error) {
	cacheKey := src.ModulePath

	modelVersionCacheMu.Lock()
	cached, hasCache := modelVersionCache[cacheKey]
	modelVersionCacheMu.Unlock()
	if hasCache && time.Since(cached.fetchedAt) < modelVersionCacheTTL {
		return append([]string(nil), cached.versions...), nil
	}

	versions, err := getGitHubTagVersions(src)
	if err != nil {
		log.Warn().Msgf("Failed to get the '%s*' tags of %s/%s from GitHub, trying the Go module proxy : [%v]", src.TagPrefix, src.Owner, src.Repo, err)
		versions, err = getModuleProxyVersions(src.ModulePath)
	}
	if err == nil && len(versions) == 0 {
		err = fmt.Errorf("no tagged versions found for %s", src.ModulePath)
	}
	if err != nil {
		if hasCache {
			log.Warn().Msgf("Using the previously fetched versions of %s : [%v]", src.ModulePath, err)
			return append([]string(nil), cached.versions...), nil
		}
		return nil, err
	}

	modelVersionCacheMu.Lock()
	modelVersionCache[cacheKey] = modelVersionCacheEntry{versions: versions, fetchedAt: time.Now()}
	modelVersionCacheMu.Unlock()

	return append([]string(nil), versions...), nil
}

func getGitHubTagVersions(src modelTagSource) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/matching-refs/tags/%s", src.Owner, src.Repo, src.TagPrefix)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := modelVersionHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP status %d for %s", resp.StatusCode, url)
	}

	var refs []struct {
		Ref string `json:"ref"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&refs); err != nil {
		return nil, err
	}

	var tags []string
	for _, ref := range refs {
		tags = append(tags, strings.TrimPrefix(ref.Ref, "refs/tags/"+src.TagPrefix))
	}
	return normalizeModelVersions(tags), nil
}

func getModuleProxyVersions(modulePath string) ([]string, error) {
	url := fmt.Sprintf("https://proxy.golang.org/%s/@v/list", modulePath)
	resp, err := modelVersionHTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Go module proxy returned HTTP status %d for %s", resp.StatusCode, modulePath)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return normalizeModelVersions(strings.Fields(string(body))), nil
}

// normalizeModelVersions keeps only release tags (vMAJOR.MINOR.PATCH), removes duplicates and sorts them in ascending order.
func normalizeModelVersions(tags []string) []string {
	seen := map[string]bool{}
	var versions []string
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if !releaseVersionPattern.MatchString(tag) || seen[tag] {
			continue
		}
		seen[tag] = true
		versions = append(versions, tag)
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareModuleVersions(versions[i], versions[j]) < 0
	})
	return versions
}

// selectModelVersion returns the requested version if it is one of the supported (tagged) versions,
// or the latest supported version if no version is requested.
func selectModelVersion(requestedVersion string, supportedVersions []string) (string, error) {
	if len(supportedVersions) == 0 {
		return "", fmt.Errorf("no supported versions are available")
	}

	requestedVersion = strings.TrimSpace(requestedVersion)
	if requestedVersion != "" {
		for _, version := range supportedVersions {
			if version == requestedVersion {
				return version, nil
			}
		}
		return "", fmt.Errorf("version %q is not a tagged version (supported: %s)", requestedVersion, strings.Join(supportedVersions, ", "))
	}

	latestVersion := supportedVersions[0]
	for _, version := range supportedVersions[1:] {
		if compareModuleVersions(version, latestVersion) > 0 {
			latestVersion = version
		}
	}
	return latestVersion, nil
}

func compareModuleVersions(left, right string) int {
	leftParts := strings.Split(strings.TrimPrefix(left, "v"), ".")
	rightParts := strings.Split(strings.TrimPrefix(right, "v"), ".")
	for index := 0; index < len(leftParts) || index < len(rightParts); index++ {
		leftValue, rightValue := 0, 0
		if index < len(leftParts) {
			leftValue, _ = strconv.Atoi(leftParts[index])
		}
		if index < len(rightParts) {
			rightValue, _ = strconv.Atoi(rightParts[index])
		}
		if leftValue != rightValue {
			if leftValue > rightValue {
				return 1
			}
			return -1
		}
	}
	return 0
}

// infraModelSchemaSource is where the Go source of each tagged cm-beetle/imdl version is downloaded from
// in order to validate infra models against the struct of the selected version.
var infraModelSchemaSource = modelschema.Source{
	ModulePath: infraModelTagSource.ModulePath,
	Owner:      infraModelTagSource.Owner,
	Repo:       infraModelTagSource.Repo,
	RepoSubdir: strings.TrimSuffix(infraModelTagSource.TagPrefix, "/"),
	TagPrefix:  infraModelTagSource.TagPrefix,
}

type infraModelSchemaInfo struct {
	PkgDir    string
	RootType  string
	FieldName string
}

var (
	onpremInfraModelSchema = infraModelSchemaInfo{PkgDir: "on-premise-model", RootType: "OnpremInfra", FieldName: "onpremiseInfraModel"}
	cloudInfraModelSchema  = infraModelSchemaInfo{PkgDir: "cloud-model", RootType: "RecommendedInfra", FieldName: "cloudInfraModel"}
)

// normalizeInfraModel validates the raw infra model against the Go struct (OnpremInfra or RecommendedInfra)
// of the given cm-beetle/imdl tagged version and returns it as that struct would be serialized.
// On failure, it also returns the HTTP status code to respond with.
func normalizeInfraModel(ctx context.Context, version string, isCloudModel bool, raw json.RawMessage) (json.RawMessage, int, error) {
	schema := onpremInfraModelSchema
	if isCloudModel {
		schema = cloudInfraModelSchema
	}

	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, http.StatusBadRequest, fmt.Errorf("'%s' is required", schema.FieldName)
	}

	pkg, err := modelschema.LoadPackage(ctx, infraModelSchemaSource, version, schema.PkgDir)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to load the cm-beetle/imdl %s '%s' model : [%v]", version, schema.PkgDir, err)
	}
	if !pkg.HasType(schema.RootType) {
		return nil, http.StatusBadRequest, fmt.Errorf("'%s' struct does not exist in cm-beetle/imdl %s '%s'. Use another version", schema.RootType, version, schema.PkgDir)
	}

	normalized, err := pkg.Normalize(raw, schema.RootType, schema.FieldName)
	if err != nil {
		var validationErr *modelschema.ValidationError
		if errors.As(err, &validationErr) {
			return nil, http.StatusBadRequest, fmt.Errorf("'%s' does not match the '%s' struct of cm-beetle/imdl %s : [%v]", schema.FieldName, schema.RootType, version, err)
		}
		return nil, http.StatusInternalServerError, err
	}
	log.Info().Msgf("Validated '%s' against the '%s' struct of cm-beetle/imdl %s", schema.FieldName, schema.RootType, version)
	return normalized, 0, nil
}

// normalizeInfraModelWithCompiledStruct decodes the raw infra model with the cm-beetle/imdl version compiled into
// cm-damselfly. It is used only for existing models whose version is not a cm-beetle/imdl tag.
func normalizeInfraModelWithCompiledStruct(isCloudModel bool, raw json.RawMessage) (json.RawMessage, int, error) {
	var target interface{} = new(onpremisemodel.OnpremInfra)
	if isCloudModel {
		target = new(cloudmodel.RecommendedInfra)
	}
	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 {
		if err := json.Unmarshal(trimmed, target); err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("invalid infra model : [%v]", err)
		}
	}
	normalized, err := json.Marshal(target)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return normalized, 0, nil
}

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
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cloudmodel "github.com/cloud-barista/cm-beetle/imdl/cloud-model"
	onpremisemodel "github.com/cloud-barista/cm-beetle/imdl/on-premise-model"
	rdbmsmodel "github.com/cloud-barista/cm-beetle/imdl/rdbms-model"
	storagemodel "github.com/cloud-barista/cm-beetle/imdl/storage-model"
	"github.com/cloud-barista/cm-damselfly/pkg/modelschema"
	softwaremodel "github.com/cloud-barista/cm-grasshopper/smdl"
	echo "github.com/labstack/echo/v4"
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

// minRDBMSModelVersion is the oldest cm-beetle/imdl tagged version which has the 'rdbms-model' package.
const minRDBMSModelVersion = "v0.1.13"

// getRDBMSModelVersions returns the tagged versions of cm-beetle/imdl which have the RDBMS model, in ascending order.
func getRDBMSModelVersions() ([]string, error) {
	infraVersions, err := getInfraModelVersions()
	if err != nil {
		return nil, err
	}
	versions := filterRDBMSModelVersions(infraVersions)
	if len(versions) == 0 {
		return nil, fmt.Errorf("no cm-beetle/imdl tagged versions with the RDBMS model (%s or later) found", minRDBMSModelVersion)
	}
	return versions, nil
}

// filterRDBMSModelVersions returns the cm-beetle/imdl versions which have the RDBMS model (minRDBMSModelVersion or later).
func filterRDBMSModelVersions(infraVersions []string) []string {
	versions := []string{}
	for _, version := range infraVersions {
		if compareModuleVersions(version, minRDBMSModelVersion) >= 0 {
			versions = append(versions, version)
		}
	}
	return versions
}

// minObjectStorageModelVersion is the oldest cm-beetle/imdl tagged version which has the 'storage-model' package.
const minObjectStorageModelVersion = "v0.1.3"

// getObjectStorageModelVersions returns the tagged versions of cm-beetle/imdl which have the object storage model, in ascending order.
func getObjectStorageModelVersions() ([]string, error) {
	infraVersions, err := getInfraModelVersions()
	if err != nil {
		return nil, err
	}
	versions := filterObjectStorageModelVersions(infraVersions)
	if len(versions) == 0 {
		return nil, fmt.Errorf("no cm-beetle/imdl tagged versions with the object storage model (%s or later) found", minObjectStorageModelVersion)
	}
	return versions, nil
}

// filterObjectStorageModelVersions returns the cm-beetle/imdl versions which have the object storage model
// (minObjectStorageModelVersion or later).
func filterObjectStorageModelVersions(infraVersions []string) []string {
	versions := []string{}
	for _, version := range infraVersions {
		if compareModuleVersions(version, minObjectStorageModelVersion) >= 0 {
			versions = append(versions, version)
		}
	}
	return versions
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
		return "", fmt.Errorf("No supported versions available")
	}

	requestedVersion = strings.TrimSpace(requestedVersion)
	if requestedVersion != "" {
		for _, version := range supportedVersions {
			if version == requestedVersion {
				return version, nil
			}
		}
		return "", fmt.Errorf("Unsupported version '%s' (supported: %s)", requestedVersion, strings.Join(supportedVersions, ", "))
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
		return nil, http.StatusBadRequest, fmt.Errorf("'%s' required", schema.FieldName)
	}

	pkg, err := modelschema.LoadPackage(ctx, infraModelSchemaSource, version, schema.PkgDir)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to load the cm-beetle/imdl %s '%s' model", version, schema.PkgDir)
		return nil, http.StatusInternalServerError, fmt.Errorf("Model schema for version %s unavailable", version)
	}
	if !pkg.HasType(schema.RootType) {
		return nil, http.StatusBadRequest, fmt.Errorf("Version %s does not support this model: use another version", version)
	}

	normalized, err := pkg.Normalize(raw, schema.RootType, schema.FieldName)
	if err != nil {
		var validationErr *modelschema.ValidationError
		if errors.As(err, &validationErr) {
			return nil, http.StatusBadRequest, fmt.Errorf("'%s' does not match version %s: %v", schema.FieldName, version, validationErr)
		}
		log.Error().Err(err).Msgf("Failed to normalize '%s'", schema.FieldName)
		return nil, http.StatusInternalServerError, fmt.Errorf("Model processing failed")
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
			log.Warn().Err(err).Msg("Failed to decode the infra model")
			return nil, http.StatusBadRequest, fmt.Errorf("Malformed infra model: check JSON syntax and field types")
		}
	}
	normalized, err := json.Marshal(target)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal the infra model")
		return nil, http.StatusInternalServerError, fmt.Errorf("Model processing failed")
	}
	return normalized, 0, nil
}

// softwareModelSchemaSource is where the Go source of each tagged cm-grasshopper/smdl version is downloaded from
// in order to validate software models against the struct of the selected version.
var softwareModelSchemaSource = modelschema.Source{
	ModulePath: softwareModelTagSource.ModulePath,
	Owner:      softwareModelTagSource.Owner,
	Repo:       softwareModelTagSource.Repo,
	RepoSubdir: strings.TrimSuffix(softwareModelTagSource.TagPrefix, "/"),
	TagPrefix:  softwareModelTagSource.TagPrefix,
}

type softwareModelSchemaInfo struct {
	RootType  string
	FieldName string
}

// The software model structs are declared in the root package of the cm-grasshopper/smdl module.
var (
	sourceSoftwareModelSchema = softwareModelSchemaInfo{RootType: "SourceGroupSoftwareProperty", FieldName: "sourceSoftwareModel"}
	targetSoftwareModelSchema = softwareModelSchemaInfo{RootType: "TargetGroupSoftwareProperty", FieldName: "targetSoftwareModel"}
)

// softwareModelSchemaOf returns the software model struct used for the given model stage.
// A source or planning model follows 'SourceGroupSoftwareProperty', and a target model follows 'TargetGroupSoftwareProperty'.
func softwareModelSchemaOf(stage ModelStage) softwareModelSchemaInfo {
	if stage == StageTarget {
		return targetSoftwareModelSchema
	}
	return sourceSoftwareModelSchema
}

// normalizeSoftwareModel validates the raw software model against the Go struct (SourceGroupSoftwareProperty or
// TargetGroupSoftwareProperty) of the given cm-grasshopper/smdl tagged version and returns it as that struct would be serialized.
// On failure, it also returns the HTTP status code to respond with.
func normalizeSoftwareModel(ctx context.Context, version string, stage ModelStage, raw json.RawMessage) (json.RawMessage, int, error) {
	schema := softwareModelSchemaOf(stage)

	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, http.StatusBadRequest, fmt.Errorf("'%s' required when 'modelStage' is '%s'", schema.FieldName, stage)
	}

	pkg, err := modelschema.LoadPackage(ctx, softwareModelSchemaSource, version, "")
	if err != nil {
		log.Error().Err(err).Msgf("Failed to load the cm-grasshopper/smdl %s model", version)
		return nil, http.StatusInternalServerError, fmt.Errorf("Model schema for version %s unavailable", version)
	}
	if !pkg.HasType(schema.RootType) {
		return nil, http.StatusBadRequest, fmt.Errorf("Version %s does not support this model: use another version", version)
	}

	normalized, err := pkg.Normalize(raw, schema.RootType, schema.FieldName)
	if err != nil {
		var validationErr *modelschema.ValidationError
		if errors.As(err, &validationErr) {
			return nil, http.StatusBadRequest, fmt.Errorf("'%s' does not match version %s: %v", schema.FieldName, version, validationErr)
		}
		log.Error().Err(err).Msgf("Failed to normalize '%s'", schema.FieldName)
		return nil, http.StatusInternalServerError, fmt.Errorf("Model processing failed")
	}
	log.Info().Msgf("Validated '%s' against the '%s' struct of cm-grasshopper/smdl %s", schema.FieldName, schema.RootType, version)
	return normalized, 0, nil
}

// normalizeSoftwareModelWithCompiledStruct decodes the raw software model with the cm-grasshopper/smdl version compiled into
// cm-damselfly. It is used only for existing models whose version is not a cm-grasshopper/smdl tag.
func normalizeSoftwareModelWithCompiledStruct(stage ModelStage, raw json.RawMessage) (json.RawMessage, int, error) {
	schema := softwareModelSchemaOf(stage)
	var target interface{} = new(softwaremodel.SourceGroupSoftwareProperty)
	if stage == StageTarget {
		target = new(softwaremodel.TargetGroupSoftwareProperty)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, http.StatusBadRequest, fmt.Errorf("'%s' required when 'modelStage' is '%s'", schema.FieldName, stage)
	}
	if err := json.Unmarshal(trimmed, target); err != nil {
		log.Warn().Err(err).Msg("Failed to decode the software model")
		return nil, http.StatusBadRequest, fmt.Errorf("Malformed software model: check JSON syntax and field types")
	}
	normalized, err := json.Marshal(target)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal the software model")
		return nil, http.StatusInternalServerError, fmt.Errorf("Model processing failed")
	}
	return normalized, 0, nil
}

// The RDBMS model structs are declared in the 'rdbms-model' package of the cm-beetle/imdl module, which is
// downloaded from the same source as the infra models.
const rdbmsModelPkgDir = "rdbms-model"

type pkgModelSchemaInfo struct {
	RootType  string
	FieldName string
}

var (
	sourceRDBMSModelSchema = pkgModelSchemaInfo{RootType: "SourceRDBMS", FieldName: "sourceRDBMSModel"}
	targetRDBMSModelSchema = pkgModelSchemaInfo{RootType: "RecommendedRDBMS", FieldName: "targetRDBMSModel"}
)

// rdbmsModelSchemaOf returns the RDBMS model struct used for the given model stage.
// A source or planning model follows 'SourceRDBMS', and a target model follows 'RecommendedRDBMS'.
func rdbmsModelSchemaOf(stage ModelStage) pkgModelSchemaInfo {
	if stage == StageTarget {
		return targetRDBMSModelSchema
	}
	return sourceRDBMSModelSchema
}

// normalizeRDBMSModel validates the raw RDBMS model against the Go struct (SourceRDBMS or RecommendedRDBMS)
// of the given cm-beetle/imdl tagged version and returns it as that struct would be serialized.
// On failure, it also returns the HTTP status code to respond with.
func normalizeRDBMSModel(ctx context.Context, version string, stage ModelStage, raw json.RawMessage) (json.RawMessage, int, error) {
	schema := rdbmsModelSchemaOf(stage)

	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, http.StatusBadRequest, fmt.Errorf("'%s' required when 'modelStage' is '%s'", schema.FieldName, stage)
	}

	pkg, err := modelschema.LoadPackage(ctx, infraModelSchemaSource, version, rdbmsModelPkgDir)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to load the cm-beetle/imdl %s '%s' model", version, rdbmsModelPkgDir)
		return nil, http.StatusInternalServerError, fmt.Errorf("Model schema for version %s unavailable", version)
	}
	if !pkg.HasType(schema.RootType) {
		return nil, http.StatusBadRequest, fmt.Errorf("Version %s does not support this model: use another version", version)
	}

	normalized, err := pkg.Normalize(raw, schema.RootType, schema.FieldName)
	if err != nil {
		var validationErr *modelschema.ValidationError
		if errors.As(err, &validationErr) {
			return nil, http.StatusBadRequest, fmt.Errorf("'%s' does not match version %s: %v", schema.FieldName, version, validationErr)
		}
		log.Error().Err(err).Msgf("Failed to normalize '%s'", schema.FieldName)
		return nil, http.StatusInternalServerError, fmt.Errorf("Model processing failed")
	}
	log.Info().Msgf("Validated '%s' against the '%s' struct of cm-beetle/imdl %s", schema.FieldName, schema.RootType, version)
	return normalized, 0, nil
}

// normalizeRDBMSModelWithCompiledStruct decodes the raw RDBMS model with the cm-beetle/imdl version compiled into
// cm-damselfly. It is used only for existing models whose version is not a cm-beetle/imdl tag with the RDBMS model.
func normalizeRDBMSModelWithCompiledStruct(stage ModelStage, raw json.RawMessage) (json.RawMessage, int, error) {
	schema := rdbmsModelSchemaOf(stage)
	var target interface{} = new(rdbmsmodel.SourceRDBMS)
	if stage == StageTarget {
		target = new(rdbmsmodel.RecommendedRDBMS)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, http.StatusBadRequest, fmt.Errorf("'%s' required when 'modelStage' is '%s'", schema.FieldName, stage)
	}
	if err := json.Unmarshal(trimmed, target); err != nil {
		log.Warn().Err(err).Msg("Failed to decode the RDBMS model")
		return nil, http.StatusBadRequest, fmt.Errorf("Malformed RDBMS model: check JSON syntax and field types")
	}
	normalized, err := json.Marshal(target)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal the RDBMS model")
		return nil, http.StatusInternalServerError, fmt.Errorf("Model processing failed")
	}
	return normalized, 0, nil
}

// The object storage model structs are declared in the 'storage-model' package of the cm-beetle/imdl module, which is
// downloaded from the same source as the infra models.
const objectStorageModelPkgDir = "storage-model"

var (
	sourceObjectStorageModelSchema = pkgModelSchemaInfo{RootType: "SourceObjectStorage", FieldName: "sourceObjectStorageModel"}
	targetObjectStorageModelSchema = pkgModelSchemaInfo{RootType: "RecommendedObjectStorage", FieldName: "targetObjectStorageModel"}
)

// objectStorageModelSchemaOf returns the object storage model struct used for the given model stage.
// A source or planning model follows 'SourceObjectStorage', and a target model follows 'RecommendedObjectStorage'.
func objectStorageModelSchemaOf(stage ModelStage) pkgModelSchemaInfo {
	if stage == StageTarget {
		return targetObjectStorageModelSchema
	}
	return sourceObjectStorageModelSchema
}

// normalizeObjectStorageModel validates the raw object storage model against the Go struct (SourceObjectStorage or
// RecommendedObjectStorage) of the given cm-beetle/imdl tagged version and returns it as that struct would be serialized.
// On failure, it also returns the HTTP status code to respond with.
func normalizeObjectStorageModel(ctx context.Context, version string, stage ModelStage, raw json.RawMessage) (json.RawMessage, int, error) {
	schema := objectStorageModelSchemaOf(stage)

	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, http.StatusBadRequest, fmt.Errorf("'%s' required when 'modelStage' is '%s'", schema.FieldName, stage)
	}

	pkg, err := modelschema.LoadPackage(ctx, infraModelSchemaSource, version, objectStorageModelPkgDir)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to load the cm-beetle/imdl %s '%s' model", version, objectStorageModelPkgDir)
		return nil, http.StatusInternalServerError, fmt.Errorf("Model schema for version %s unavailable", version)
	}
	if !pkg.HasType(schema.RootType) {
		return nil, http.StatusBadRequest, fmt.Errorf("Version %s does not support this model: use another version", version)
	}

	normalized, err := pkg.Normalize(raw, schema.RootType, schema.FieldName)
	if err != nil {
		var validationErr *modelschema.ValidationError
		if errors.As(err, &validationErr) {
			return nil, http.StatusBadRequest, fmt.Errorf("'%s' does not match version %s: %v", schema.FieldName, version, validationErr)
		}
		log.Error().Err(err).Msgf("Failed to normalize '%s'", schema.FieldName)
		return nil, http.StatusInternalServerError, fmt.Errorf("Model processing failed")
	}
	log.Info().Msgf("Validated '%s' against the '%s' struct of cm-beetle/imdl %s", schema.FieldName, schema.RootType, version)
	return normalized, 0, nil
}

// normalizeObjectStorageModelWithCompiledStruct decodes the raw object storage model with the cm-beetle/imdl version
// compiled into cm-damselfly. It is used only for existing models whose version is not a cm-beetle/imdl tag with the
// object storage model.
func normalizeObjectStorageModelWithCompiledStruct(stage ModelStage, raw json.RawMessage) (json.RawMessage, int, error) {
	schema := objectStorageModelSchemaOf(stage)
	var target interface{} = new(storagemodel.SourceObjectStorage)
	if stage == StageTarget {
		target = new(storagemodel.RecommendedObjectStorage)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, http.StatusBadRequest, fmt.Errorf("'%s' required when 'modelStage' is '%s'", schema.FieldName, stage)
	}
	if err := json.Unmarshal(trimmed, target); err != nil {
		log.Warn().Err(err).Msg("Failed to decode the object storage model")
		return nil, http.StatusBadRequest, fmt.Errorf("Malformed object storage model: check JSON syntax and field types")
	}
	normalized, err := json.Marshal(target)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal the object storage model")
		return nil, http.StatusInternalServerError, fmt.Errorf("Model processing failed")
	}
	return normalized, 0, nil
}

// ModelStage is the stage of a migration user model (infra, software, RDBMS or object storage) in the migration process.
type ModelStage string

const (
	StageSource   ModelStage = "source"
	StagePlanning ModelStage = "planning"
	StageTarget   ModelStage = "target"
)

func (s ModelStage) isValid() bool {
	return s == StageSource || s == StagePlanning || s == StageTarget
}

// parseModelStageParam parses the required 'modelStage' query parameter ('source', 'planning' or 'target').
func parseModelStageParam(c echo.Context) (ModelStage, error) {
	stage := ModelStage(strings.ToLower(strings.TrimSpace(c.QueryParam("modelStage"))))
	if !stage.isValid() {
		return "", fmt.Errorf("'modelStage' must be '%s', '%s' or '%s'",
			StageSource, StagePlanning, StageTarget)
	}
	return stage, nil
}

// parseModelStagesParam parses the 'modelStage' query parameter of the list APIs, which can have multiple stages given
// by commas ('modelStage=source,planning') or by repeating the parameter. If it is not given, all the stages are meant
// (an empty list is returned), or it is an error if it is required.
func parseModelStagesParam(c echo.Context, required bool) ([]ModelStage, error) {
	param := strings.Join(c.QueryParams()["modelStage"], ",")
	if strings.TrimSpace(param) == "" && !required {
		return nil, nil
	}
	stages := []ModelStage{}
	for _, value := range strings.Split(param, ",") {
		stage := ModelStage(strings.ToLower(strings.TrimSpace(value)))
		if !stage.isValid() {
			return nil, fmt.Errorf("'modelStage' must be '%s', '%s' or '%s' (separate multiple stages with commas)",
				StageSource, StagePlanning, StageTarget)
		}
		if !slices.Contains(stages, stage) {
			stages = append(stages, stage)
		}
	}
	return stages, nil
}

// storedModelStage returns the stage of a stored model. For a model stored before 'modelStage' was
// introduced, the stage is derived from 'isTargetModel' (true: target, false: source).
func storedModelStage(m map[string]interface{}) (ModelStage, bool) {
	if s, ok := m["modelStage"].(string); ok {
		if stage := ModelStage(s); stage.isValid() {
			return stage, true
		}
	}
	if isTargetModel, ok := m["isTargetModel"].(bool); ok {
		if isTargetModel {
			return StageTarget, true
		}
		return StageSource, true
	}
	return "", false
}

// decorateModel returns a copy of the stored model with the fields of the response filled in: 'category', and 'modelStage'
// and 'lineage' for a model stored before they were introduced. 'modelType' is normalized as well.
// The stored map is not modified, and 'category' is never stored.
func decorateModel(m map[string]interface{}) map[string]interface{} {
	decorated := make(map[string]interface{}, len(m)+3)
	for k, v := range m {
		decorated[k] = v
	}
	if category, ok := storedModelCategory(m); ok {
		decorated["category"] = string(category)
	}
	if stage, ok := storedModelStage(m); ok {
		decorated["modelStage"] = string(stage)
		if _, exists := m["lineage"]; !exists {
			decorated["lineage"] = storedLineage(m, stage)
		}
	}
	if m["modelType"] == formerObjectStorageModel {
		decorated["modelType"] = ObjectStorageModel
	}
	return decorated
}

// decorateRecord returns the record of a model just stored (e.g. onPremModelRecord) in the form of decorateModel.
func decorateRecord(record interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(record)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal the model")
		return nil, fmt.Errorf("Model processing failed")
	}
	var m map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber() // so that the large integers of the model body are kept as they are
	if err := decoder.Decode(&m); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal the model")
		return nil, fmt.Errorf("Model processing failed")
	}
	return decorateModel(m), nil
}

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	echo "github.com/labstack/echo/v4"
)

// The cases cover the responses which do not need the source code of a tagged version.
func TestCreateInfraModelVersionParam(t *testing.T) {
	modelVersionCacheMu.Lock()
	previous, hadPrevious := modelVersionCache[infraModelTagSource.ModulePath]
	modelVersionCache[infraModelTagSource.ModulePath] = modelVersionCacheEntry{
		versions:  []string{"v0.1.18", "v0.1.19"},
		fetchedAt: time.Now(),
	}
	modelVersionCacheMu.Unlock()
	t.Cleanup(func() {
		modelVersionCacheMu.Lock()
		defer modelVersionCacheMu.Unlock()
		if hadPrevious {
			modelVersionCache[infraModelTagSource.ModulePath] = previous
		} else {
			delete(modelVersionCache, infraModelTagSource.ModulePath)
		}
	})

	tests := []struct {
		name    string
		query   string
		wantErr string
	}{
		{"former onprem parameter", "modelType=onprem&onpremModelVersion=v0.1.19", "'onpremModelVersion' is no longer supported. Use 'modelVersion' instead"},
		{"former cloud parameter", "modelType=cloud&cloudModelVersion=v0.1.19", "'cloudModelVersion' is no longer supported. Use 'modelVersion' instead"},
		{"former parameter of the other model type", "modelType=cloud&onpremModelVersion=v0.1.19", "'onpremModelVersion' is no longer supported"},
		{"former parameter with an empty value", "modelType=onprem&cloudModelVersion=", "'cloudModelVersion' is no longer supported"},
		{"former parameter with modelVersion", "modelType=onprem&modelVersion=v0.1.19&onpremModelVersion=v0.1.19", "'onpremModelVersion' is no longer supported"},
		{"modelVersion is not a tag", "modelType=onprem&modelVersion=v9.9.9", "not a tagged version"},
		{"modelVersion for onprem", "modelType=onprem&modelVersion=v0.1.18", "'onpremiseInfraModel' is required"},
		{"modelVersion for cloud", "modelType=cloud&modelVersion=v0.1.18", "'cloudInfraModel' is required"},
		{"modelVersion omitted", "modelType=cloud", "'cloudInfraModel' is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/infra-model?modelStage=source&"+tt.query, nil)
			rec := httptest.NewRecorder()
			if err := CreateInfraModel(e.NewContext(req, rec)); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tt.wantErr) {
				t.Fatalf("body = %s, want it to contain %q", rec.Body.String(), tt.wantErr)
			}
		})
	}
}

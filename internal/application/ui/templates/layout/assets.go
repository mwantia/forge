package layout

// assetVersion is set once by UIService.PostInit via SetAssetVersion.
var assetVersion string

// SetAssetVersion injects the content-hash version string computed in the ui
// package. Must be called before any template renders.
func SetAssetVersion(v string) {
	assetVersion = v
}

// AssetURL returns the versioned URL for a named static asset. The ?v= query
// param changes on every deploy so browsers can cache responses indefinitely.
func AssetURL(name string) string {
	if assetVersion == "" {
		return "/ui/assets/" + name
	}
	return "/ui/assets/" + name + "?v=" + assetVersion
}

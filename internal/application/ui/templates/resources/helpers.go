package tmplresources

import (
	"encoding/json"
	"fmt"
	"time"

	domresource "github.com/mwantia/forge/internal/domain/resource"
)

type revisionItem struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	When    string `json:"when"`
}

type resourceItem struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Session string         `json:"session"`
	Created string         `json:"created"`
	Tags    []string       `json:"tags"`
	Level0  string         `json:"level0"`
	Level1  string         `json:"level1"`
	History []revisionItem `json:"history"`
}

func buildResourceItems(resources []*domresource.Resource, histories map[string][]*domresource.ResourceRevision) []resourceItem {
	items := make([]resourceItem, 0, len(resources))
	for _, r := range resources {
		l1 := ""
		if r.Meta.Extra != nil {
			if v, ok := r.Meta.Extra["summary"]; ok {
				if s, ok := v.(string); ok {
					l1 = s
				}
			}
		}
		if l1 == "" {
			l1 = r.Meta.Description
		}
		created := ""
		if !r.Meta.CreatedAt.IsZero() {
			created = r.Meta.CreatedAt.Format("2006-01-02 15:04")
		}
		tags := r.Meta.Tags
		if tags == nil {
			tags = []string{}
		}

		var revs []revisionItem
		for _, rev := range histories[r.ID] {
			when := relativeTime(rev.CommittedAt)
			if when == "" {
				when = "—"
			}
			revs = append(revs, revisionItem{
				Hash:    rev.Hash,
				Message: rev.CommitMessage,
				When:    when,
			})
		}
		if revs == nil {
			revs = []revisionItem{}
		}

		items = append(items, resourceItem{
			ID:      r.ID,
			Name:    r.Meta.Name,
			Type:    r.Meta.Type,
			Session: r.Meta.Session,
			Created: created,
			Tags:    tags,
			Level0:  r.Content,
			Level1:  l1,
			History: revs,
		})
	}
	return items
}

func resourcesPageData(resources []*domresource.Resource, histories map[string][]*domresource.ResourceRevision) string {
	items := buildResourceItems(resources, histories)
	itemsJSON, _ := json.Marshal(items)

	firstID := ""
	if len(items) > 0 {
		firstID = items[0].ID
	}
	firstIDJSON, _ := json.Marshal(firstID)

	return `{` +
		`resources:` + string(itemsJSON) + `,` +
		`selectedId:` + string(firstIDJSON) + `,` +
		`level:'L0',` +
		`search:'',` +
		`filterType:'',` +
		`filterSession:'',` +
		`typeColor(t){const m={memory:'#e0a45e',reference:'#86c275',online:'#6fa9d6',archive:'#9a9aa2'};return m[t]||'#9a9aa2'},` +
		`typeBadge(t){const m={memory:'rgba(224,164,94,0.13)',reference:'rgba(134,194,117,0.13)',online:'rgba(111,169,214,0.13)',archive:'rgba(255,255,255,0.07)'};return m[t]||'rgba(255,255,255,0.07)'},` +
		`get selected(){return this.resources.find(r=>r.id===this.selectedId)||null},` +
		`get filteredResources(){return this.resources.filter(r=>{` +
		`const s=this.search.toLowerCase();` +
		`if(s&&!r.name.toLowerCase().includes(s)&&!r.id.includes(s)&&!r.level0.toLowerCase().includes(s))return false;` +
		`if(this.filterType&&r.type!==this.filterType)return false;` +
		`if(this.filterSession&&r.session!==this.filterSession)return false;` +
		`return true;` +
		`})},` +
		`get displayContent(){if(!this.selected)return '';return this.level==='L0'?this.selected.level0:(this.selected.level1||'')},` +
		`selectFirst(){this.$nextTick(()=>{const v=this.filteredResources;if(v.length>0&&!v.find(r=>r.id===this.selectedId)){this.selectedId=v[0].id;}})},` +
		`copyContent(){navigator.clipboard.writeText(this.displayContent)}` +
		`}`
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func hasSessions(resources []*domresource.Resource) bool {
	for _, r := range resources {
		if r.Meta.Session != "" {
			return true
		}
	}
	return false
}

func uniqueSessions(resources []*domresource.Resource) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, r := range resources {
		if r.Meta.Session != "" {
			if _, dup := seen[r.Meta.Session]; !dup {
				seen[r.Meta.Session] = struct{}{}
				out = append(out, r.Meta.Session)
			}
		}
	}
	return out
}

func shortSession(session string) string {
	if len(session) > 8 {
		return session[:8]
	}
	return session
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2, 2006")
	}
}

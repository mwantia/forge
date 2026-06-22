package tmplresources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	domresource "github.com/mwantia/forge/internal/domain/resource"
)

var mdRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
	),
)

func renderMarkdown(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}

	var buf bytes.Buffer
	if err := mdRenderer.Convert([]byte(src), &buf); err != nil {
		return "<p>" + src + "</p>"
	}

	return buf.String()
}

type revisionItem struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	When    string `json:"when"`
}

// ResourceItem is the wire shape consumed by the UI Alpine components.
type ResourceItem struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Session  string         `json:"session"`
	Created  string         `json:"created"`
	Tags     []string       `json:"tags"`
	Score    float64        `json:"score"`
	Level0   string         `json:"level0"`
	Level1   string         `json:"level1"`
	Rendered string         `json:"rendered"`
	History  []revisionItem `json:"history"`
}

// BuildResourceItems converts domain resources + history into UI wire items.
func BuildResourceItems(resources []*domresource.Resource, histories map[string][]*domresource.ResourceRevision) []ResourceItem {
	items := make([]ResourceItem, 0, len(resources))
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

		items = append(items, ResourceItem{
			ID:       r.ID,
			Name:     r.Meta.Name,
			Type:     r.Meta.Type,
			Session:  r.Meta.Session,
			Created:  created,
			Tags:     tags,
			Score:    r.Score,
			Level0:   r.Content,
			Level1:   l1,
			Rendered: renderMarkdown(r.Content),
			History:  revs,
		})
	}
	return items
}

func resourcesPageData(resources []*domresource.Resource, histories map[string][]*domresource.ResourceRevision) string {
	items := BuildResourceItems(resources, histories)
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
		`raw:false,` +
		`search:'',` +
		`filterType:'',` +
		`filterSession:'',` +
		`mode:'direct',` +
		`recallQuery:'',` +
		`recallLimit:10,` +
		`recallMinScore:0,` +
		`recallResults:[],` +
		`recallLoading:false,` +
		`_recallTimer:null,` +
		`typeColor(t){const m={memory:'#e0a45e',reference:'#86c275',online:'#6fa9d6',archive:'#9a9aa2'};return m[t]||'#9a9aa2'},` +
		`typeBadge(t){const m={memory:'rgba(224,164,94,0.13)',reference:'rgba(134,194,117,0.13)',online:'rgba(111,169,214,0.13)',archive:'rgba(255,255,255,0.07)'};return m[t]||'rgba(255,255,255,0.07)'},` +
		`setMode(m){this.mode=m;if(m==='direct'){this.recallQuery='';this.recallResults=[];}else{this.search='';}this.selectFirst();},` +
		`onRecallInput(){clearTimeout(this._recallTimer);if(!this.recallQuery.trim()){this.recallResults=[];this.selectedId='';return;}this._recallTimer=setTimeout(()=>this.doRecall(),1000);},` +
		`async doRecall(){if(!this.recallQuery.trim())return;this.recallLoading=true;try{const r=await fetch('/ui/resources/recall',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({query:this.recallQuery,limit:this.recallLimit})});const d=await r.json();this.recallResults=d.resources||[];this.selectFirst();}catch(e){console.error(e);}finally{this.recallLoading=false;}},` +
		`get activeList(){return this.mode==='recall'?this.recallResults:this.resources},` +
		`get selected(){return this.activeList.find(r=>r.id===this.selectedId)||null},` +
		`setSelected(id){this.selectedId=id;this.raw=false},` +
		`get filteredResources(){return this.activeList.filter(r=>{` +
		`if(this.mode==='direct'){const s=this.search.toLowerCase();if(s&&!r.name.toLowerCase().includes(s)&&!r.id.includes(s)&&!r.level0.toLowerCase().includes(s))return false;}` +
		`else{if(r.score<this.recallMinScore)return false;}` +
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

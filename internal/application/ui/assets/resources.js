document.addEventListener('alpine:init', () => {
  Alpine.data('resourcesPage', (init) => ({
    resources: init.resources,
    selectedId: init.selectedId,

    // view state
    level: Alpine.$persist('L0').as('forge:resources:level'),
    raw: false,
    showHistory: Alpine.$persist(false).as('forge:resources:showHistory'),
    showContent: Alpine.$persist(true).as('forge:resources:showContent'),

    // direct mode
    search: '',
    filterTypes: Alpine.$persist(['memory', 'reference', 'online']).as('forge:resources:filterTypes'),
    filterSession: '',

    // recall mode
    mode: Alpine.$persist('direct').as('forge:resources:mode'),
    recallQuery: '',
    recallLimit: Alpine.$persist(10).as('forge:resources:recallLimit'),
    recallMinScore: Alpine.$persist(0).as('forge:resources:recallMinScore'),
    recallResults: [],
    recallLoading: false,
    _recallTimer: null,

    // tag input
    newTag: '',

    // upload modal
    uploadOpen: false,
    uploadName: '',
    uploadType: 'memory',
    uploadTags: [],
    newUploadTag: '',
    uploadCommit: '',
    uploadContent: '',
    uploadWorking: false,
    uploadError: '',

    // edit mode
    editing: false,
    editContent: '',
    editMessage: '',
    editWorking: false,
    editError: '',

    // ── type colours ────────────────────────────────────────────────────────

    typeColor(t) {
      const m = { memory: '#e0a45e', reference: '#86c275', online: '#6fa9d6', archive: '#9a9aa2' };
      return m[t] || '#9a9aa2';
    },

    typeBadge(t) {
      const m = {
        memory:    'rgba(224,164,94,0.13)',
        reference: 'rgba(134,194,117,0.13)',
        online:    'rgba(111,169,214,0.13)',
        archive:   'rgba(255,255,255,0.07)',
      };
      return m[t] || 'rgba(255,255,255,0.07)';
    },

    // ── filter / mode ────────────────────────────────────────────────────────

    toggleType(t) {
      const i = this.filterTypes.indexOf(t);
      if (i >= 0) {
        this.filterTypes.splice(i, 1);
      } else {
        this.filterTypes.push(t);
      }
      this.selectFirst();
    },

    setMode(m) {
      this.mode = m;
      if (m === 'direct') {
        this.recallQuery = '';
        this.recallResults = [];
      } else {
        this.search = '';
      }
      this.selectFirst();
    },

    // ── recall ───────────────────────────────────────────────────────────────

    onRecallInput() {
      clearTimeout(this._recallTimer);
      if (!this.recallQuery.trim()) {
        this.recallResults = [];
        this.selectedId = '';
        return;
      }
      this._recallTimer = setTimeout(() => this.doRecall(), 1000);
    },

    async doRecall() {
      if (!this.recallQuery.trim()) return;
      this.recallLoading = true;
      try {
        const r = await fetch('/ui/resources/recall', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query: this.recallQuery, limit: this.recallLimit }),
        });
        const d = await r.json();
        this.recallResults = d.resources || [];
        this.selectFirst();
      } catch (e) {
        console.error(e);
      } finally {
        this.recallLoading = false;
      }
    },

    // ── computed lists ───────────────────────────────────────────────────────

    get activeList() {
      return this.mode === 'recall' ? this.recallResults : this.resources;
    },

    get selected() {
      return this.activeList.find(r => r.id === this.selectedId) || null;
    },

    get filteredResources() {
      return this.activeList.filter(r => {
        if (this.mode === 'direct') {
          const s = this.search.toLowerCase();
          if (s && !r.name.toLowerCase().includes(s) && !r.id.includes(s) && !r.level0.toLowerCase().includes(s)) {
            return false;
          }
        } else {
          if (r.score < this.recallMinScore) return false;
        }
        if (this.filterTypes.length > 0 && !this.filterTypes.includes(r.type)) return false;
        if (this.filterSession && r.session !== this.filterSession) return false;
        return true;
      });
    },

    get displayContent() {
      if (!this.selected) return '';
      return this.level === 'L0' ? this.selected.level0 : (this.selected.level1 || '');
    },

    // ── selection ────────────────────────────────────────────────────────────

    setSelected(id) {
      this.selectedId = id;
      this.raw = false;
      this.editing = false;
      this.newTag = '';
    },

    selectFirst() {
      this.$nextTick(() => {
        const v = this.filteredResources;
        if (v.length > 0 && !v.find(r => r.id === this.selectedId)) {
          this.selectedId = v[0].id;
        }
      });
    },

    copyContent() {
      navigator.clipboard.writeText(this.displayContent);
    },

    // ── tags (detail view) ───────────────────────────────────────────────────

    addTag(t) {
      t = t.trim();
      if (!t) return;
      const r = this.resources.find(x => x.id === this.selectedId);
      if (r && !r.tags.includes(t)) {
        r.tags = [...r.tags, t];
        this.doSaveTags(r.tags);
      }
      this.newTag = '';
    },

    removeTag(t) {
      const r = this.resources.find(x => x.id === this.selectedId);
      if (r) {
        r.tags = r.tags.filter(x => x !== t);
        this.doSaveTags(r.tags);
      }
    },

    async doSaveTags(tags) {
      if (!this.selectedId) return;
      try {
        await fetch('/ui/resources/' + this.selectedId + '/meta', {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ tags }),
        });
      } catch (e) {
        console.error(e);
      }
    },

    onNewTagKey(e) {
      if (e.key === ' ' || e.key === 'Enter') {
        e.preventDefault();
        this.addTag(this.newTag);
      }
    },

    // ── upload tags ──────────────────────────────────────────────────────────

    addUploadTag(t) {
      t = t.trim();
      if (t && !this.uploadTags.includes(t)) {
        this.uploadTags = [...this.uploadTags, t];
      }
      this.newUploadTag = '';
    },

    removeUploadTag(t) {
      this.uploadTags = this.uploadTags.filter(x => x !== t);
    },

    onUploadTagKey(e) {
      if (e.key === ' ' || e.key === 'Enter') {
        e.preventDefault();
        this.addUploadTag(this.newUploadTag);
      }
    },

    // ── delete ───────────────────────────────────────────────────────────────

    async doDelete() {
      if (!this.selected) return;
      if (!confirm('Delete "' + this.selected.name + '"? This cannot be undone.')) return;
      try {
        const r = await fetch('/ui/resources/' + this.selected.id, { method: 'DELETE' });
        if (!r.ok) { alert('Delete failed.'); return; }
        this.resources = this.resources.filter(x => x.id !== this.selectedId);
        const rem = this.filteredResources;
        this.selectedId = rem.length > 0 ? rem[0].id : '';
      } catch (e) {
        alert('Delete failed.');
      }
    },

    // ── upload ───────────────────────────────────────────────────────────────

    openUpload() {
      this.uploadOpen    = true;
      this.uploadName    = '';
      this.uploadTags    = [];
      this.newUploadTag  = '';
      this.uploadCommit  = '';
      this.uploadContent = '';
      this.uploadError   = '';
    },

    closeUpload() {
      this.uploadOpen = false;
    },

    onFileChange(e) {
      const f = e.target?.files?.[0] || e.dataTransfer?.files?.[0];
      if (!f) return;
      if (f.size > 1048576) { this.uploadError = 'File exceeds 1 MiB limit.'; return; }
      this.uploadError = '';
      this.uploadName  = f.name.replace(/\.(md|txt)$/i, '');
      const fr = new FileReader();
      fr.onload = ev => { this.uploadContent = ev.target.result; };
      fr.readAsText(f);
    },

    async doUpload() {
      if (!this.uploadContent || !this.uploadName) return;
      this.uploadWorking = true;
      this.uploadError   = '';
      try {
        const r = await fetch('/ui/resources/upload', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            content:        this.uploadContent,
            commit_message: this.uploadCommit,
            meta: { name: this.uploadName, type: this.uploadType, tags: this.uploadTags },
          }),
        });
        if (!r.ok) { const d = await r.json(); this.uploadError = d.error || 'Upload failed.'; return; }
        location.reload();
      } catch (e) {
        this.uploadError = 'Upload failed.';
      } finally {
        this.uploadWorking = false;
      }
    },

    // ── download ─────────────────────────────────────────────────────────────

    doDownload() {
      if (!this.selected) return;
      const b = new Blob([this.selected.level0], { type: 'text/markdown' });
      const u = URL.createObjectURL(b);
      const a = document.createElement('a');
      a.href     = u;
      a.download = (this.selected.name || this.selected.id) + '.md';
      a.click();
      URL.revokeObjectURL(u);
    },

    // ── edit / commit ────────────────────────────────────────────────────────

    startEdit() {
      this.editing     = true;
      this.editContent = this.selected?.level0 || '';
      this.editMessage = '';
      this.editError   = '';
    },

    cancelEdit() {
      this.editing   = false;
      this.editError = '';
    },

    async doCommit() {
      if (!this.selected || !this.editContent) return;
      this.editWorking = true;
      this.editError   = '';
      try {
        const r = await fetch('/ui/resources/' + this.selected.id + '/commit', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ content: this.editContent, commit_message: this.editMessage }),
        });
        if (!r.ok) { const d = await r.json(); this.editError = d.error || 'Commit failed.'; return; }
        const updated = await r.json();
        const idx = this.resources.findIndex(x => x.id === this.selectedId);
        if (idx >= 0) {
          this.resources[idx] = {
            ...this.resources[idx],
            level0:   updated.level0,
            rendered: updated.rendered,
            history:  updated.history,
          };
        }
        this.editing = false;
      } catch (e) {
        this.editError = 'Commit failed.';
      } finally {
        this.editWorking = false;
      }
    },
  }));
});

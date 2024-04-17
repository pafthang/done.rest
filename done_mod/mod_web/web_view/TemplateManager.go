package webview

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	modweb "github.com/hiveot/hub/done_mod/mod_web"
)

const baseTemplateName = "base.gohtml"
const TemplateExt = "*.gohtml"

var TM *TemplateManager

// TemplateManager manages parsing and executing of application templates
// It has 2 modes: embedded for production and dynamic for development
// In embedded mode it parses templates once from the embedded file system.
// In dynamic mode the templates are parsed from templatePath before being executed.
type TemplateManager struct {
	templatePath string
	allTemplates *template.Template
	//
	renderMux sync.Mutex
}

// GetTemplate returns the requested template ready for execution.
// Returns an error if the template doesn't exist.
func (svc *TemplateManager) GetTemplate(name string) (*template.Template, error) {
	if svc.templatePath == "" {

		// in case of using the embedded templates, clone is required before execution
		tpl := svc.allTemplates.Lookup(name)
		if tpl == nil {
			err := fmt.Errorf("template '%s' not found", name)
			return nil, err
		}
		tpl, err := tpl.Clone()
		if err != nil {
			err = fmt.Errorf("clone template failed: %w", err)
		}
		return tpl, err
	}

	// Reparse all templates
	// TODO-1: parse only the files that are needed, but how to know which ones are?
	// TODO-2: parse only if files have changed
	slog.Info("GetTemplate, parsing files")
	t := template.New("hiveot")
	templateFS := os.DirFS(svc.templatePath)
	err := svc.parseTemplateFiles(t, templateFS)
	tpl := t.Lookup(name)
	if tpl == nil {
		return nil, errors.New("Template '" + name + "' not found.")
	}
	return tpl, err
}

// ParseAllTemplates reads all application html templates
func (svc *TemplateManager) ParseAllTemplates() {
	var err error
	slog.Info("Parsing all templates")
	t := template.New("hiveot")

	// embed app templates first to allow components to override templates in block statements
	if svc.templatePath == "" {
		err = svc.parseTemplateFiles(t, modweb.EmbeddedViews)
	} else {
		// live filesystem for development
		templateFS := os.DirFS(svc.templatePath)
		err = svc.parseTemplateFiles(t, templateFS)
	}
	if err != nil {
		slog.Error("Error parsing templates: ", "err", err.Error())
	}
	svc.allTemplates = t
}

// ParseTemplateFiles parses the html templates of the given filesystem and updates
// the given template collection.
func (svc *TemplateManager) parseTemplateFiles(t *template.Template, files fs.FS) error {
	err := fs.WalkDir(files, ".", func(parent string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			subT, err := t.ParseFS(files, filepath.Join(parent, TemplateExt))
			_ = subT
			// only report template errors, but keep going
			if err != nil && !strings.HasPrefix(err.Error(), "template: pattern matches no files:") {
				slog.Error("error parsing template", "err", err)
			}
			err = nil
		}
		return err
	})
	return err
}

// RenderTemplate renders a requested template.
//
// To support progressive rendering, the base template is only used if
// the request does not have the 'HX-Request' header.
//
// If the HX-Request header is not present then the full page is rendered
// where the template of name is embedded in the 'base' layout. The base layout
// contains the html and body tags with an 'embed' statement which will be
// replaced with the template of the given name.
//
// With the HX-Request header, the page is injected in the request target
// (usually the Body element) without a full page reload.
//
//	w is the writer to render to
//	name is the name of the template to render
//	data contains a map of variables to pass to the template renderer
func (svc *TemplateManager) RenderTemplate(w http.ResponseWriter, r *http.Request, name string, data any) {
	isFragment := r.Header.Get("HX-Request") != ""
	isBoosted := r.Header.Get("HX-Boosted") != ""
	if isFragment && !isBoosted {
		svc.RenderFragment(w, name, data)
	} else {
		svc.RenderFull(w, name, data)
	}
}

// RenderFragment renders the template 'name' with the given data without base template.
// Intended to be used with hx-target pointing to the page in which to render the fragment.
//
//	w is the writer to render to
//	name is the name of the template to render
//	data is a map with template variables and their values
func (svc *TemplateManager) RenderFragment(w http.ResponseWriter, name string, data any) {
	slog.Info("RenderPartial", "template", name)
	svc.renderMux.Lock()
	defer svc.renderMux.Unlock()
	tpl, err := svc.GetTemplate(name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error(err.Error())
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	err = tpl.Execute(w, data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("rendering template failed", "err", err)
		_, _ = w.Write([]byte("template render error: " + err.Error()))
		return
	}
}

// RenderFull embeds the template 'name' into the base template and executes.
// The base template contains the 'embed' field where the template is injected into.
//
// If a template has an error, the error is returned to the user instead along with a 500 error.
//
//	 w is the writer to render into
//		t is the template bundle to lookup base and name
//		name is the name of the template to render
//		data contains a map of variables to pass to the template renderer
func (svc *TemplateManager) RenderFull(w http.ResponseWriter, name string, data any) {
	slog.Info("RenderFull", "template", name)
	svc.renderMux.Lock()
	defer svc.renderMux.Unlock()

	baseT, err := svc.GetTemplate(baseTemplateName)
	//baseT := svc.allTemplates.Lookup(baseTemplateName)
	if baseT == nil {
		// filesystem incorrect?uh oh
		slog.Error("Can't read the base template.",
			"templateFile", baseTemplateName,
			"err", err)
		_, _ = w.Write([]byte("template not found: " + baseTemplateName))
		return
	}

	tpl, err := svc.GetTemplate(name)
	if err == nil {
		// This is where the magic happens: replace the 'embed' template with the given template.
		_, err = baseT.AddParseTree("embed", tpl.Tree)
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("template error: " + err.Error()))
		return
	}
	err = baseT.Execute(w, data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("rendering template failed", "err", err)
		_, _ = w.Write([]byte("template render error: " + err.Error()))
		return
	}
}

// RenderWithOverlay embeds the template 'name' into the overlay template and executes.
// The base template uses the 'embed' field.
// If a template has an error, the error is returned to the user instead along with a 500 error.
//
//	name is the name of the template to render
//	overlay is the optional overlay to use. "" for the default overlay layout.html.
//func RenderWithOverlay(w http.ResponseWriter, t *template.Template, name string, overlay string, data map[string]any) {
//	renderMux.Lock()
//	defer renderMux.Unlock()
//
//	if overlay == "" {
//		overlay = "layout.html"
//	}
//	overlayT := t.Lookup(overlay)
//	overlayT, err := overlayT.Clone()
//	if err != nil {
//		w.WriteHeader(http.StatusInternalServerError)
//		slog.Error("Cloning overlay failed", "err", err)
//		_, _ = w.Write([]byte("overlay error: " + err.Error()))
//		return
//	}
//	tpl := t.Lookup(name)
//	if tpl == nil || tpl.Tree == nil {
//		err = errors.New("missing or invalid template: " + name)
//	} else {
//		// problem with error "cannot Clone ... after it has executed"
//		tpl, err = tpl.Clone()
//		// This is where the magic happens: replace the 'embed' template with the given template.
//		if err == nil {
//			_, err = overlayT.AddParseTree("embed", tpl.Tree)
//		}
//	}
//	if err != nil {
//		w.WriteHeader(http.StatusInternalServerError)
//		slog.Error("merging templates failed", "err", err)
//		_, _ = w.Write([]byte("template error: " + err.Error()))
//		return
//	}
//	err = overlayT.Execute(w, data)
//	if err != nil {
//		w.WriteHeader(http.StatusInternalServerError)
//		slog.Error("rendering template failed", "err", err)
//		_, _ = w.Write([]byte("template render error: " + err.Error()))
//		return
//	}
//}

// InitTemplateManager initializes the template manager singleton
// templatePath provides a path to a live filesystem where the templates resides.
// If provided templates are read and parsed live before rendering.
// This is optional. Use "" in production.
func InitTemplateManager(templatePath string) *TemplateManager {
	//
	TM = &TemplateManager{templatePath: templatePath}
	return TM
}

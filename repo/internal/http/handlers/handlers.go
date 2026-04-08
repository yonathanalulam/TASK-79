package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fleetcommerce/internal/alerts"
	"fleetcommerce/internal/audit"
	"fleetcommerce/internal/auth"
	"fleetcommerce/internal/cart"
	"fleetcommerce/internal/catalog"
	"fleetcommerce/internal/http/middleware"
	"fleetcommerce/internal/http/views"
	templviews "fleetcommerce/internal/http/views/templ"
	"fleetcommerce/internal/imports"
	"fleetcommerce/internal/metrics"
	"fleetcommerce/internal/notifications"
	"fleetcommerce/internal/orders"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handlers struct {
	authSvc    *auth.Service
	catalogSvc *catalog.Service
	cartSvc    *cart.Service
	orderSvc   *orders.Service
	notifSvc   *notifications.Service
	alertSvc   *alerts.Service
	metricSvc  *metrics.Service
	auditSvc   *audit.Service
	importSvc  *imports.Service
	renderer   *views.Renderer
	uploadsDir string
	maxUpload  int64
}

func New(
	authSvc *auth.Service,
	catalogSvc *catalog.Service,
	cartSvc *cart.Service,
	orderSvc *orders.Service,
	notifSvc *notifications.Service,
	alertSvc *alerts.Service,
	metricSvc *metrics.Service,
	auditSvc *audit.Service,
	importSvc *imports.Service,
	renderer *views.Renderer,
	uploadsDir string,
	maxUpload int64,
) *Handlers {
	return &Handlers{
		authSvc: authSvc, catalogSvc: catalogSvc, cartSvc: cartSvc,
		orderSvc: orderSvc, notifSvc: notifSvc, alertSvc: alertSvc,
		metricSvc: metricSvc, auditSvc: auditSvc, importSvc: importSvc,
		renderer: renderer, uploadsDir: uploadsDir, maxUpload: maxUpload,
	}
}

// Helper types
type Pagination struct {
	Page       int
	TotalPages int
	Total      int
}

type Flash struct {
	Type    string
	Message string
}

func (h *Handlers) baseData(c *gin.Context, title, activeNav string) gin.H {
	user := middleware.GetUser(c.Request.Context())
	perms := middleware.GetPermissions(c.Request.Context())
	roles := middleware.GetRoles(c.Request.Context())
	csrf := middleware.GetCSRFToken(c)

	var unread int
	if user != nil {
		unread, _ = h.notifSvc.CountUnread(c.Request.Context(), user.ID)
	}

	data := gin.H{
		"Title":       title,
		"ActiveNav":   activeNav,
		"User":        user,
		"Permissions": perms,
		"Roles":       roles,
		"CSRFToken":   csrf,
		"UnreadCount": unread,
	}

	if flash, _ := c.Cookie("flash_msg"); flash != "" {
		ftype, _ := c.Cookie("flash_type")
		if ftype == "" {
			ftype = "success"
		}
		data["Flash"] = Flash{Type: ftype, Message: flash}
		c.SetCookie("flash_msg", "", -1, "/", "", false, false)
		c.SetCookie("flash_type", "", -1, "/", "", false, false)
	}
	return data
}

func (h *Handlers) templData(c *gin.Context, title, activeNav string) templviews.PageData {
	user := middleware.GetUser(c.Request.Context())
	perms := middleware.GetPermissions(c.Request.Context())
	roles := middleware.GetRoles(c.Request.Context())
	csrf := middleware.GetCSRFToken(c)
	var unread int
	userName := ""
	roleName := ""
	if user != nil {
		userName = user.FullName
		if h.notifSvc != nil {
			unread, _ = h.notifSvc.CountUnread(c.Request.Context(), user.ID)
		}
		if len(roles) > 0 {
			roleName = roles[0]
		}
	}
	pd := templviews.PageData{
		Title:       title,
		ActiveNav:   activeNav,
		UserName:    userName,
		RoleName:    roleName,
		CSRFToken:   csrf,
		Permissions: perms,
		UnreadCount: unread,
	}
	if flash, _ := c.Cookie("flash_msg"); flash != "" {
		ftype, _ := c.Cookie("flash_type")
		if ftype == "" {
			ftype = "success"
		}
		pd.FlashType = ftype
		pd.FlashMsg = flash
		c.SetCookie("flash_msg", "", -1, "/", "", false, false)
		c.SetCookie("flash_type", "", -1, "/", "", false, false)
	}
	return pd
}

func (h *Handlers) setFlash(c *gin.Context, ftype, msg string) {
	c.SetCookie("flash_msg", msg, 5, "/", "", false, false)
	c.SetCookie("flash_type", ftype, 5, "/", "", false, false)
}

func jsonOK(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": msg, "data": data})
}

func jsonErr(c *gin.Context, status int, msg string, errs interface{}) {
	c.JSON(status, gin.H{"ok": false, "message": msg, "errors": errs})
}

func getUser(c *gin.Context) *auth.User {
	return middleware.GetUser(c.Request.Context())
}

func getIntParam(c *gin.Context, name string) int {
	v, _ := strconv.Atoi(c.Param(name))
	return v
}

func getIntQuery(c *gin.Context, name string, def int) int {
	v, err := strconv.Atoi(c.Query(name))
	if err != nil {
		return def
	}
	return v
}

// ====== Scope policy helpers ======

// hasGlobalCartRead returns true if the user has broad cart/order visibility.
// This is the single source of truth for global-read scope.
// Administrator: has users.manage
// Auditor: has audit.read (read-only but can see all)
// Inventory manager with order.transition: can see all orders/carts for fulfillment
func (h *Handlers) hasGlobalCartRead(perms map[string]bool) bool {
	return perms["users.manage"] || perms["audit.read"]
}

func (h *Handlers) hasGlobalOrderRead(perms map[string]bool) bool {
	return perms["users.manage"] || perms["audit.read"] || perms["order.transition"]
}

// ====== Object-level authorization helpers ======

// enforceCartAccess checks the current user can access the given cart.
func (h *Handlers) enforceCartAccess(c *gin.Context, cartObj *cart.Cart) bool {
	user := getUser(c)
	perms := middleware.GetPermissions(c.Request.Context())
	if h.hasGlobalCartRead(perms) {
		return true
	}
	if cartObj.CreatedBy != nil && *cartObj.CreatedBy == user.ID {
		return true
	}
	jsonErr(c, http.StatusForbidden, "Access denied to this cart", nil)
	c.Abort()
	return false
}

// enforceOrderAccess checks the current user can access the given order.
// Admins, inventory managers (order.transition), auditors can see all.
// Sales associates can see orders they created.
func (h *Handlers) enforceOrderAccess(c *gin.Context, order *orders.Order) bool {
	user := getUser(c)
	perms := middleware.GetPermissions(c.Request.Context())
	if h.hasGlobalOrderRead(perms) {
		return true
	}
	if order.CreatedBy != nil && *order.CreatedBy == user.ID {
		return true
	}
	jsonErr(c, http.StatusForbidden, "Access denied to this order", nil)
	c.Abort()
	return false
}

// loadAndAuthorizeCart loads a cart by ID and enforces access. Returns nil if denied.
func (h *Handlers) loadAndAuthorizeCart(c *gin.Context) *cart.Cart {
	id := getIntParam(c, "id")
	cartObj, err := h.cartSvc.GetCart(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, http.StatusNotFound, "Cart not found", nil)
		c.Abort()
		return nil
	}
	if !h.enforceCartAccess(c, cartObj) {
		return nil
	}
	return cartObj
}

// loadAndAuthorizeOrder loads an order by ID and enforces access. Returns nil if denied.
func (h *Handlers) loadAndAuthorizeOrder(c *gin.Context) *orders.Order {
	id := getIntParam(c, "id")
	order, err := h.orderSvc.GetOrder(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, http.StatusNotFound, "Order not found", nil)
		c.Abort()
		return nil
	}
	if !h.enforceOrderAccess(c, order) {
		return nil
	}
	return order
}

// ====== Auth ======

func (h *Handlers) LoginPage(c *gin.Context) {
	pd := h.templData(c, "Login", "")
	views.RenderTempl(c, http.StatusOK, templviews.LoginPage(pd, "", ""))
}

func (h *Handlers) LoginPost(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	session, _, err := h.authSvc.Login(c.Request.Context(), username, password)
	if err != nil {
		pd := h.templData(c, "Login", "")
		views.RenderTempl(c, http.StatusOK, templviews.LoginPage(pd, "Invalid username or password", username))
		return
	}

	c.SetCookie("session_id", session.ID, int(24*time.Hour/time.Second), "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

func (h *Handlers) Logout(c *gin.Context) {
	sessionID := middleware.GetSessionID(c.Request.Context())
	if sessionID != "" {
		h.authSvc.DestroySession(c.Request.Context(), sessionID)
	}
	c.SetCookie("session_id", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

func (h *Handlers) GetMe(c *gin.Context) {
	user := getUser(c)
	roles := middleware.GetRoles(c.Request.Context())
	perms := middleware.GetPermissions(c.Request.Context())
	jsonOK(c, "ok", gin.H{"user": user, "roles": roles, "permissions": perms})
}

// ====== Dashboard ======

func (h *Handlers) DashboardPage(c *gin.Context) {
	ctx := c.Request.Context()
	user := getUser(c)

	openCarts, _ := h.cartSvc.CountOpen(ctx)
	activeOrders, _ := h.orderSvc.CountActive(ctx)
	unreadNotifs, _ := h.notifSvc.CountUnread(ctx, user.ID)
	activeAlerts, _ := h.alertSvc.CountActive(ctx)

	pd := h.templData(c, "Dashboard", "dashboard")
	summary := templviews.DashboardSummary{
		OpenCarts: openCarts, ActiveOrders: activeOrders,
		UnreadNotifications: unreadNotifs, ActiveAlerts: activeAlerts,
	}
	views.RenderTempl(c, http.StatusOK, templviews.DashboardPage(pd, summary))
}

func (h *Handlers) DashboardSummaryAPI(c *gin.Context) {
	ctx := c.Request.Context()
	user := getUser(c)
	openCarts, _ := h.cartSvc.CountOpen(ctx)
	activeOrders, _ := h.orderSvc.CountActive(ctx)
	unreadNotifs, _ := h.notifSvc.CountUnread(ctx, user.ID)
	activeAlerts, _ := h.alertSvc.CountActive(ctx)
	jsonOK(c, "ok", gin.H{
		"open_carts":           openCarts,
		"active_orders":        activeOrders,
		"unread_notifications": unreadNotifs,
		"active_alerts":        activeAlerts,
	})
}

// ====== Catalog ======

func (h *Handlers) CatalogPage(c *gin.Context) {
	ctx := c.Request.Context()
	brands, _ := h.catalogSvc.ListBrands(ctx)

	brandID := getIntQuery(c, "brand_id", 0)
	seriesID := getIntQuery(c, "series_id", 0)
	status := c.Query("status")
	query := c.Query("q")
	page := getIntQuery(c, "page", 1)

	models, total, _ := h.catalogSvc.ListModels(ctx, catalog.ListParams{
		BrandID: brandID, SeriesID: seriesID, Status: status, Query: query, Page: page, PageSize: 20,
	})

	totalPages := int(math.Ceil(float64(total) / 20))

	pd := h.templData(c, "Vehicle Catalog", "catalog")
	var brandOpts []templviews.BrandOption
	for _, b := range brands {
		brandOpts = append(brandOpts, templviews.BrandOption{ID: b.ID, Name: b.Name})
	}
	var catModels []templviews.CatalogModel
	for _, m := range models {
		catModels = append(catModels, templviews.CatalogModel{
			ID: m.ID, ModelCode: m.ModelCode, ModelName: m.ModelName,
			BrandName: m.BrandName, SeriesName: m.SeriesName,
			Year: m.Year, StockQuantity: m.StockQuantity, PublicationStatus: m.PublicationStatus,
		})
	}
	pag := templviews.Pagination{Page: page, TotalPages: totalPages}
	views.RenderTempl(c, http.StatusOK, templviews.CatalogPage(pd, brandOpts, catModels, pag, status, query))
}

func (h *Handlers) CatalogDetailPage(c *gin.Context) {
	ctx := c.Request.Context()
	id := getIntParam(c, "id")
	model, err := h.catalogSvc.GetModel(ctx, id)
	if err != nil {
		h.setFlash(c, "error", "Vehicle not found")
		c.Redirect(http.StatusFound, "/catalog")
		return
	}

	media, _ := h.catalogSvc.ListMedia(ctx, id)
	versions, _ := h.catalogSvc.ListVersions(ctx, id)

	data := h.baseData(c, model.ModelName, "catalog")
	data["Model"] = model
	data["Media"] = media
	data["Versions"] = versions
	h.renderer.HTML(c, http.StatusOK, "catalog_detail", data)
}

func (h *Handlers) CatalogEditPage(c *gin.Context) {
	ctx := c.Request.Context()
	brands, _ := h.catalogSvc.ListBrands(ctx)

	data := h.baseData(c, "Edit Vehicle", "catalog")
	data["Brands"] = brands
	data["Errors"] = gin.H{}

	idStr := c.Param("id")
	if idStr == "" || idStr == "new" {
		data["IsNew"] = true
		data["Model"] = catalog.VehicleModel{}
		data["Title"] = "New Vehicle"
	} else {
		id, _ := strconv.Atoi(idStr)
		model, err := h.catalogSvc.GetModel(ctx, id)
		if err != nil {
			c.Redirect(http.StatusFound, "/catalog")
			return
		}
		data["IsNew"] = false
		data["Model"] = model
		seriesList, _ := h.catalogSvc.ListSeries(ctx, model.BrandID)
		data["SeriesList"] = seriesList
	}

	h.renderer.HTML(c, http.StatusOK, "catalog_edit", data)
}

func (h *Handlers) CreateModelAPI(c *gin.Context) {
	user := getUser(c)
	brandID, _ := strconv.Atoi(c.PostForm("brand_id"))
	seriesID, _ := strconv.Atoi(c.PostForm("series_id"))
	year, _ := strconv.Atoi(c.PostForm("year"))
	stock, _ := strconv.Atoi(c.PostForm("stock_quantity"))

	id, err := h.catalogSvc.CreateModel(c.Request.Context(), catalog.CreateModelParams{
		BrandID: brandID, SeriesID: seriesID, ModelCode: c.PostForm("model_code"),
		ModelName: c.PostForm("model_name"), Year: year, Description: c.PostForm("description"),
		StockQuantity: stock, ExpiryDate: c.PostForm("expiry_date"),
	}, user.ID)

	if err != nil {
		h.setFlash(c, "error", "Failed to create vehicle: "+err.Error())
		c.Redirect(http.StatusFound, "/catalog/new")
		return
	}

	h.setFlash(c, "success", "Vehicle created successfully")
	c.Redirect(http.StatusFound, fmt.Sprintf("/catalog/%d", id))
}

func (h *Handlers) UpdateDraftAPI(c *gin.Context) {
	user := getUser(c)
	id := getIntParam(c, "id")
	year, _ := strconv.Atoi(c.PostForm("year"))
	stock, _ := strconv.Atoi(c.PostForm("stock_quantity"))

	err := h.catalogSvc.UpdateDraft(c.Request.Context(), id, catalog.UpdateDraftParams{
		ModelName: c.PostForm("model_name"), Year: year, Description: c.PostForm("description"),
		StockQuantity: stock, ExpiryDate: c.PostForm("expiry_date"),
	}, user.ID)

	if err != nil {
		h.setFlash(c, "error", "Failed to update draft: "+err.Error())
	} else {
		h.setFlash(c, "success", "Draft updated successfully")
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/catalog/%d", id))
}

func (h *Handlers) PublishModelAPI(c *gin.Context) {
	user := getUser(c)
	id := getIntParam(c, "id")
	if err := h.catalogSvc.Publish(c.Request.Context(), id, user.ID); err != nil {
		h.setFlash(c, "error", "Failed to publish: "+err.Error())
	} else {
		h.setFlash(c, "success", "Vehicle published successfully")
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/catalog/%d", id))
}

func (h *Handlers) UnpublishModelAPI(c *gin.Context) {
	user := getUser(c)
	id := getIntParam(c, "id")
	if err := h.catalogSvc.Unpublish(c.Request.Context(), id, user.ID); err != nil {
		h.setFlash(c, "error", "Failed to unpublish: "+err.Error())
	} else {
		h.setFlash(c, "success", "Vehicle unpublished")
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/catalog/%d", id))
}

func (h *Handlers) ListBrandsAPI(c *gin.Context) {
	brands, _ := h.catalogSvc.ListBrands(c.Request.Context())
	jsonOK(c, "ok", brands)
}

func (h *Handlers) ListSeriesAPI(c *gin.Context) {
	brandID := getIntQuery(c, "brand_id", 0)
	list, _ := h.catalogSvc.ListSeries(c.Request.Context(), brandID)

	// For HTMX select update
	if c.GetHeader("HX-Request") == "true" {
		html := `<option value="">All Series</option>`
		for _, s := range list {
			html += fmt.Sprintf(`<option value="%d">%s</option>`, s.ID, s.Name)
		}
		c.Data(http.StatusOK, "text/html", []byte(html))
		return
	}
	jsonOK(c, "ok", list)
}

func (h *Handlers) ListModelsAPI(c *gin.Context) {
	models, total, _ := h.catalogSvc.ListModels(c.Request.Context(), catalog.ListParams{
		BrandID:  getIntQuery(c, "brand_id", 0),
		SeriesID: getIntQuery(c, "series_id", 0),
		Status:   c.Query("status"),
		Query:    c.Query("q"),
		Page:     getIntQuery(c, "page", 1),
		PageSize: 20,
	})
	jsonOK(c, "ok", gin.H{"models": models, "total": total})
}

func (h *Handlers) GetModelAPI(c *gin.Context) {
	id := getIntParam(c, "id")
	model, err := h.catalogSvc.GetModel(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, 404, "Not found", nil)
		return
	}
	jsonOK(c, "ok", model)
}

// ====== Media Upload ======

func (h *Handlers) UploadMediaAPI(c *gin.Context) {
	user := getUser(c)
	modelID := getIntParam(c, "id")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		jsonErr(c, 400, "No file uploaded", nil)
		return
	}
	defer file.Close()

	if header.Size > h.maxUpload {
		jsonErr(c, 400, fmt.Sprintf("File too large (max %d MB)", h.maxUpload/(1024*1024)), nil)
		return
	}

	// Validate extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	validExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".mp4": true, ".webm": true, ".mov": true}
	if !validExts[ext] {
		jsonErr(c, 400, "File extension not allowed", nil)
		return
	}

	// Read content for MIME sniffing (do not trust client Content-Type header)
	content, err := io.ReadAll(file)
	if err != nil {
		jsonErr(c, 500, "Failed to read file", nil)
		return
	}
	sniffedMIME := http.DetectContentType(content)
	allowed := map[string]string{
		"image/jpeg": "image", "image/png": "image", "image/gif": "image", "image/webp": "image",
		"video/mp4": "video", "video/webm": "video",
		"application/octet-stream": "", // fallback for formats DetectContentType doesn't know
	}
	kind, ok := allowed[sniffedMIME]
	if !ok {
		jsonErr(c, 400, fmt.Sprintf("File content type %q not allowed", sniffedMIME), nil)
		return
	}
	if kind == "" { // octet-stream fallback: infer from extension
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
			kind = "image"
		case ".mp4", ".webm", ".mov":
			kind = "video"
		default:
			jsonErr(c, 400, "Cannot determine file type", nil)
			return
		}
	}
	mimeType := sniffedMIME

	hash := sha256.Sum256(content)
	fingerprint := hex.EncodeToString(hash[:])

	// Store file
	storedName := uuid.New().String() + ext
	storedDir := filepath.Join(h.uploadsDir, fmt.Sprintf("%d", modelID))
	os.MkdirAll(storedDir, 0755)
	storedPath := filepath.Join(storedDir, storedName)

	if err := os.WriteFile(storedPath, content, 0644); err != nil {
		jsonErr(c, 500, "Failed to store file", nil)
		return
	}

	relativePath := fmt.Sprintf("%d/%s", modelID, storedName)

	_, err = h.catalogSvc.CreateMedia(c.Request.Context(), catalog.Media{
		VehicleModelID:    modelID,
		Kind:              kind,
		OriginalFilename:  header.Filename,
		StoredPath:        relativePath,
		MimeType:          mimeType,
		SizeBytes:         header.Size,
		SHA256Fingerprint: fingerprint,
		UploadedBy:        &user.ID,
	})

	if err != nil {
		jsonErr(c, 500, "Failed to save media record", nil)
		return
	}

	h.setFlash(c, "success", "Media uploaded successfully")
	c.Redirect(http.StatusFound, fmt.Sprintf("/catalog/%d", modelID))
}

// ====== CSV Export/Import ======

func (h *Handlers) ExportCatalogCSV(c *gin.Context) {
	models, _, _ := h.catalogSvc.ListModels(c.Request.Context(), catalog.ListParams{PageSize: 10000})

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=catalog_export.csv")

	c.Writer.WriteString("model_code,model_name,brand,series,year,description,publication_status,stock_quantity,expiry_date\n")
	for _, m := range models {
		c.Writer.WriteString(fmt.Sprintf("%s,%s,%s,%s,%d,%s,%s,%d,%s\n",
			m.ModelCode, m.ModelName, m.BrandName, m.SeriesName, m.Year, m.Description, m.PublicationStatus, m.StockQuantity, m.ExpiryDate))
	}
}

func (h *Handlers) ImportCatalogCSV(c *gin.Context) {
	user := getUser(c)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		jsonErr(c, 400, "No file uploaded", nil)
		return
	}
	defer file.Close()

	job, rows, err := h.importSvc.ParseAndValidate(c.Request.Context(), header.Filename, file, user.ID)
	if err != nil {
		jsonErr(c, 400, "Import failed: "+err.Error(), nil)
		return
	}

	// Return preview HTML for HTMX
	if c.GetHeader("HX-Request") == "true" {
		html := fmt.Sprintf(`<h4>Import Preview: %s</h4><p>Total: %d | Valid: %d | Invalid: %d</p>`,
			job.Filename, job.TotalRows, job.ValidRows, job.InvalidRows)
		html += `<table class="table"><thead><tr><th>Row</th><th>Status</th><th>Data</th><th>Errors</th></tr></thead><tbody>`
		for _, r := range rows {
			errStr := ""
			if r.Errors != nil {
				errStr = string(r.Errors)
			}
			statusClass := "valid"
			if r.Status == "invalid" {
				statusClass = "invalid"
			}
			html += fmt.Sprintf(`<tr class="row-%s"><td>%d</td><td><span class="badge badge-%s">%s</span></td><td><code class="small-json">%s</code></td><td>%s</td></tr>`,
				statusClass, r.RowNumber, r.Status, r.Status, string(r.RawData), errStr)
		}
		html += `</tbody></table>`
		if job.ValidRows > 0 {
			html += fmt.Sprintf(`<form method="POST" action="/api/catalog/imports/%d/commit"><input type="hidden" name="csrf_token" value="%s"><button type="submit" class="btn btn-success">Commit %d Valid Rows</button></form>`,
				job.ID, middleware.GetCSRFToken(c), job.ValidRows)
		}
		c.Data(http.StatusOK, "text/html", []byte(html))
		return
	}

	jsonOK(c, "Import validated", gin.H{"job": job, "rows": rows})
}

func (h *Handlers) GetImportJob(c *gin.Context) {
	jobID := getIntParam(c, "job_id")
	job, err := h.importSvc.GetJob(c.Request.Context(), jobID)
	if err != nil {
		jsonErr(c, 404, "Job not found", nil)
		return
	}
	rows, _ := h.importSvc.GetJobRows(c.Request.Context(), jobID)
	jsonOK(c, "ok", gin.H{"job": job, "rows": rows})
}

func (h *Handlers) CommitImportJob(c *gin.Context) {
	user := getUser(c)
	jobID := getIntParam(c, "job_id")
	if err := h.importSvc.CommitJob(c.Request.Context(), jobID, user.ID); err != nil {
		h.setFlash(c, "error", "Commit failed: "+err.Error())
	} else {
		h.setFlash(c, "success", "Import committed successfully")
	}
	c.Redirect(http.StatusFound, "/catalog")
}

func (h *Handlers) ImportModal(c *gin.Context) {
	data := h.baseData(c, "Import", "catalog")
	h.renderer.HTMLFragment(c, http.StatusOK, "csv-import-modal", data)
}

// ====== Cart ======

func (h *Handlers) CartPage(c *gin.Context) {
	user := getUser(c)
	perms := middleware.GetPermissions(c.Request.Context())
	carts, _ := h.cartSvc.ListCarts(c.Request.Context(), user.ID, h.hasGlobalCartRead(perms))

	pd := h.templData(c, "Shopping Carts", "cart")
	var cartViews []templviews.CartView
	for _, ct := range carts {
		cartViews = append(cartViews, templviews.CartView{
			ID: ct.ID, CustomerName: ct.CustomerName, ItemCount: ct.ItemCount,
			Status: ct.Status, CreatedAt: ct.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	views.RenderTempl(c, http.StatusOK, templviews.CartListPage(pd, cartViews))
}

func (h *Handlers) CartDetailPage(c *gin.Context) {
	id := getIntParam(c, "id")
	cartObj, err := h.cartSvc.GetCart(c.Request.Context(), id)
	if err != nil {
		c.Redirect(http.StatusFound, "/cart")
		return
	}
	if !h.enforceCartAccess(c, cartObj) {
		return
	}
	items, _ := h.cartSvc.ListCartItems(c.Request.Context(), id)

	pd := h.templData(c, fmt.Sprintf("Cart #%d", id), "cart")
	cv := templviews.CartView{ID: cartObj.ID, CustomerName: cartObj.CustomerName, ItemCount: cartObj.ItemCount, Status: cartObj.Status, CreatedAt: cartObj.CreatedAt.Format("2006-01-02 15:04")}
	var itemViews []templviews.CartItemView
	for _, it := range items {
		price := ""
		if it.UnitPriceSnapshot != nil {
			price = fmt.Sprintf("%.2f", *it.UnitPriceSnapshot)
		}
		itemViews = append(itemViews, templviews.CartItemView{
			ID: it.ID, VehicleName: it.VehicleName, Quantity: it.Quantity,
			UnitPriceSnapshot: price, ValidityStatus: it.ValidityStatus, ValidationMessage: it.ValidationMessage,
		})
	}
	views.RenderTempl(c, http.StatusOK, templviews.CartDetailPage(pd, cv, itemViews))
}

func (h *Handlers) CreateCartAPI(c *gin.Context) {
	user := getUser(c)
	custID, _ := strconv.Atoi(c.PostForm("customer_account_id"))
	var custPtr *int
	if custID > 0 {
		custPtr = &custID
	}
	id, err := h.cartSvc.CreateCart(c.Request.Context(), custPtr, user.ID)
	if err != nil {
		jsonErr(c, 500, "Failed to create cart", nil)
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/cart/%d", id))
}

func (h *Handlers) ListCartsAPI(c *gin.Context) {
	user := getUser(c)
	perms := middleware.GetPermissions(c.Request.Context())
	carts, _ := h.cartSvc.ListCarts(c.Request.Context(), user.ID, h.hasGlobalCartRead(perms))
	jsonOK(c, "ok", carts)
}

func (h *Handlers) GetCartAPI(c *gin.Context) {
	id := getIntParam(c, "id")
	cartObj, err := h.cartSvc.GetCart(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, 404, "Cart not found", nil)
		return
	}
	if !h.enforceCartAccess(c, cartObj) {
		return
	}
	items, _ := h.cartSvc.ListCartItems(c.Request.Context(), id)
	jsonOK(c, "ok", gin.H{"cart": cartObj, "items": items})
}

func (h *Handlers) AddCartItemAPI(c *gin.Context) {
	cartObj := h.loadAndAuthorizeCart(c)
	if cartObj == nil {
		return
	}
	vehicleID, _ := strconv.Atoi(c.PostForm("vehicle_model_id"))
	qty, _ := strconv.Atoi(c.PostForm("quantity"))
	price, _ := strconv.ParseFloat(c.PostForm("unit_price"), 64)
	if qty <= 0 {
		qty = 1
	}
	_, err := h.cartSvc.AddItem(c.Request.Context(), cartObj.ID, cart.AddItemParams{
		VehicleModelID: vehicleID, Quantity: qty, UnitPrice: price,
	})
	if err != nil {
		jsonErr(c, 400, "Failed to add item: "+err.Error(), nil)
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/cart/%d", cartObj.ID))
}

func (h *Handlers) UpdateCartItemAPI(c *gin.Context) {
	cartObj := h.loadAndAuthorizeCart(c)
	if cartObj == nil {
		return
	}
	itemID := getIntParam(c, "item_id")
	qty, _ := strconv.Atoi(c.PostForm("quantity"))
	if qty <= 0 {
		jsonErr(c, 400, "Quantity must be > 0", nil)
		return
	}
	if err := h.cartSvc.UpdateItem(c.Request.Context(), cartObj.ID, itemID, qty); err != nil {
		jsonErr(c, http.StatusNotFound, "Item not found in this cart", nil)
		return
	}
	jsonOK(c, "Item updated", nil)
}

func (h *Handlers) DeleteCartItemAPI(c *gin.Context) {
	cartObj := h.loadAndAuthorizeCart(c)
	if cartObj == nil {
		return
	}
	itemID := getIntParam(c, "item_id")
	if err := h.cartSvc.DeleteItem(c.Request.Context(), cartObj.ID, itemID); err != nil {
		jsonErr(c, http.StatusNotFound, "Item not found in this cart", nil)
		return
	}
	items, _ := h.cartSvc.ListCartItems(c.Request.Context(), cartObj.ID)
	if c.GetHeader("HX-Request") == "true" {
		jsonOK(c, "Item removed", items)
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/cart/%d", cartObj.ID))
}

func (h *Handlers) RevalidateCartAPI(c *gin.Context) {
	cartObj := h.loadAndAuthorizeCart(c)
	if cartObj == nil {
		return
	}
	items, err := h.cartSvc.ValidateCart(c.Request.Context(), cartObj.ID)
	if err != nil {
		jsonErr(c, 500, "Validation failed: "+err.Error(), nil)
		return
	}
	h.setFlash(c, "success", "Cart revalidated")
	if c.GetHeader("HX-Request") != "true" {
		c.Redirect(http.StatusFound, fmt.Sprintf("/cart/%d", cartObj.ID))
		return
	}
	jsonOK(c, "Validated", items)
}

func (h *Handlers) MergeCartAPI(c *gin.Context) {
	// Authorize target cart
	cartObj := h.loadAndAuthorizeCart(c)
	if cartObj == nil {
		return
	}
	// Authorize source cart
	sourceID, _ := strconv.Atoi(c.PostForm("source_cart_id"))
	sourceCart, err := h.cartSvc.GetCart(c.Request.Context(), sourceID)
	if err != nil {
		h.setFlash(c, "error", "Source cart not found")
		c.Redirect(http.StatusFound, fmt.Sprintf("/cart/%d", cartObj.ID))
		return
	}
	if !h.enforceCartAccess(c, sourceCart) {
		return
	}

	user := getUser(c)
	targetID := cartObj.ID
	result, err := h.cartSvc.MergeCart(c.Request.Context(), targetID, sourceID, user.ID)
	if err != nil {
		h.setFlash(c, "error", "Merge failed: "+err.Error())
	} else {
		h.setFlash(c, "success", fmt.Sprintf("Merged: %d items combined, %d items added", result.ItemsMerged, result.ItemsAdded))
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/cart/%d", targetID))
}

func (h *Handlers) MergeCartModal(c *gin.Context) {
	// Authorize target cart
	cartObj := h.loadAndAuthorizeCart(c)
	if cartObj == nil {
		return
	}
	if cartObj.CustomerAccountID == nil {
		c.Data(200, "text/html", []byte(`<div class="flash flash-error">Cart has no customer assigned</div>`))
		return
	}
	// List candidates, then filter by ownership/scope
	allCandidates, _ := h.cartSvc.ListOpenCartsByCustomer(c.Request.Context(), *cartObj.CustomerAccountID, cartObj.ID)
	user := getUser(c)
	perms := middleware.GetPermissions(c.Request.Context())
	isGlobalRead := h.hasGlobalCartRead(perms)
	var mergeable []cart.Cart
	for _, cand := range allCandidates {
		if isGlobalRead || (cand.CreatedBy != nil && *cand.CreatedBy == user.ID) {
			mergeable = append(mergeable, cand)
		}
	}
	data := h.baseData(c, "Merge Cart", "cart")
	data["CartID"] = cartObj.ID
	data["MergeableCarts"] = mergeable
	h.renderer.HTMLFragment(c, http.StatusOK, "merge-cart-modal", data)
}

func (h *Handlers) CheckoutCartAPI(c *gin.Context) {
	authCart := h.loadAndAuthorizeCart(c)
	if authCart == nil {
		return
	}
	user := getUser(c)
	cartID := authCart.ID

	items, cartObj, err := h.cartSvc.Checkout(c.Request.Context(), cartID, user.ID)
	if err != nil {
		h.setFlash(c, "error", "Checkout failed: "+err.Error())
		c.Redirect(http.StatusFound, fmt.Sprintf("/cart/%d", cartID))
		return
	}

	// Create order from cart
	var lines []orders.CreateOrderLineParams
	for _, item := range items {
		if item.ValidityStatus != "valid" {
			continue
		}
		lines = append(lines, orders.CreateOrderLineParams{
			VehicleModelID:       item.VehicleModelID,
			QuantityRequested:    item.Quantity,
			PublicationSnapshot:  item.ValidityStatus,
		})
	}

	orderID, orderNum, err := h.orderSvc.CreateOrder(c.Request.Context(), orders.CreateOrderParams{
		CustomerAccountID: *cartObj.CustomerAccountID,
		SourceCartID:      cartID,
		CreatedBy:         user.ID,
		Lines:             lines,
	})
	if err != nil {
		h.setFlash(c, "error", "Order creation failed: "+err.Error())
		c.Redirect(http.StatusFound, fmt.Sprintf("/cart/%d", cartID))
		return
	}

	h.setFlash(c, "success", fmt.Sprintf("Order %s created successfully", orderNum))
	c.Redirect(http.StatusFound, fmt.Sprintf("/orders/%d", orderID))
}

// ====== Orders ======

func (h *Handlers) OrdersPage(c *gin.Context) {
	user := getUser(c)
	perms := middleware.GetPermissions(c.Request.Context())
	page := getIntQuery(c, "page", 1)
	ordersList, total, _ := h.orderSvc.ListOrders(c.Request.Context(), orders.ListParams{
		Status: c.Query("status"), Query: c.Query("q"), Page: page, PageSize: 20,
		ViewerUserID: user.ID, GlobalReadScope: h.hasGlobalOrderRead(perms),
	})
	totalPages := int(math.Ceil(float64(total) / 20))

	pd := h.templData(c, "Orders", "orders")
	var ov []templviews.OrderView
	for _, o := range ordersList {
		ov = append(ov, templviews.OrderView{
			ID: o.ID, OrderNumber: o.OrderNumber, CustomerName: o.CustomerName,
			Status: o.Status, CreatedAt: o.CreatedAt.Format("2006-01-02 15:04"), PromisedDate: o.PromisedDate,
		})
	}
	pag := templviews.Pagination{Page: page, TotalPages: totalPages}
	views.RenderTempl(c, http.StatusOK, templviews.OrdersPage(pd, ov, pag, c.Query("status"), c.Query("q")))
}

func (h *Handlers) OrderDetailPage(c *gin.Context) {
	id := getIntParam(c, "id")
	order, err := h.orderSvc.GetOrder(c.Request.Context(), id)
	if err != nil {
		c.Redirect(http.StatusFound, "/orders")
		return
	}
	if !h.enforceOrderAccess(c, order) {
		return
	}

	lines, _ := h.orderSvc.ListOrderLines(c.Request.Context(), id)
	notes, _ := h.orderSvc.ListNotes(c.Request.Context(), id)
	history, _ := h.orderSvc.ListHistory(c.Request.Context(), id)
	allowed := h.orderSvc.GetAllowedTransitions(order.Status)

	pd := h.templData(c, "Order "+order.OrderNumber, "orders")
	ov := templviews.OrderView{ID: order.ID, OrderNumber: order.OrderNumber, CustomerName: order.CustomerName, Status: order.Status, CreatedAt: order.CreatedAt.Format("2006-01-02 15:04:05")}
	var lv []templviews.OrderLineView
	for _, l := range lines {
		lv = append(lv, templviews.OrderLineView{VehicleName: l.VehicleName, QuantityRequested: l.QuantityRequested, QuantityAllocated: l.QuantityAllocated, QuantityBackord: l.QuantityBackordered, LineStatus: l.LineStatus})
	}
	var nv []templviews.OrderNoteView
	for _, n := range notes {
		nv = append(nv, templviews.OrderNoteView{NoteType: n.NoteType, Content: n.Content, AuthorName: n.AuthorName, CreatedAt: n.CreatedAt.Format("2006-01-02 15:04")})
	}
	var hv []templviews.TimelineEntry
	for _, h := range history {
		hv = append(hv, templviews.TimelineEntry{FromStatus: h.FromStatus, ToStatus: h.ToStatus, ActorName: h.ActorName, ActorType: h.ActorType, Reason: h.Reason, Time: h.TransitionedAt.Format("2006-01-02 15:04:05")})
	}
	views.RenderTempl(c, http.StatusOK, templviews.OrderDetailPage(pd, ov, lv, nv, hv, allowed))
}

func (h *Handlers) ListOrdersAPI(c *gin.Context) {
	user := getUser(c)
	perms := middleware.GetPermissions(c.Request.Context())
	ordersList, total, _ := h.orderSvc.ListOrders(c.Request.Context(), orders.ListParams{
		Status: c.Query("status"), Query: c.Query("q"), Page: getIntQuery(c, "page", 1), PageSize: 20,
		ViewerUserID: user.ID, GlobalReadScope: h.hasGlobalOrderRead(perms),
	})
	jsonOK(c, "ok", gin.H{"orders": ordersList, "total": total})
}

func (h *Handlers) GetOrderAPI(c *gin.Context) {
	id := getIntParam(c, "id")
	order, err := h.orderSvc.GetOrder(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, 404, "Order not found", nil)
		return
	}
	if !h.enforceOrderAccess(c, order) {
		return
	}
	jsonOK(c, "ok", order)
}

func (h *Handlers) TransitionOrderAPI(c *gin.Context) {
	order := h.loadAndAuthorizeOrder(c)
	if order == nil {
		return
	}
	user := getUser(c)
	toStatus := c.PostForm("to_status")

	err := h.orderSvc.TransitionOrder(c.Request.Context(), order.ID, toStatus, &user.ID, "user", "")
	if err != nil {
		h.setFlash(c, "error", "Transition failed: "+err.Error())
	} else {
		h.setFlash(c, "success", fmt.Sprintf("Order transitioned to %s", toStatus))
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/orders/%d", order.ID))
}

func (h *Handlers) RecordPaymentAPI(c *gin.Context) {
	order := h.loadAndAuthorizeOrder(c)
	if order == nil {
		return
	}
	user := getUser(c)
	err := h.orderSvc.RecordPayment(c.Request.Context(), order.ID, user.ID)
	if err != nil {
		h.setFlash(c, "error", "Payment recording failed: "+err.Error())
	} else {
		h.setFlash(c, "success", "Payment recorded")
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/orders/%d", order.ID))
}

func (h *Handlers) AddOrderNoteAPI(c *gin.Context) {
	order := h.loadAndAuthorizeOrder(c)
	if order == nil {
		return
	}
	user := getUser(c)
	noteType := c.PostForm("note_type")
	content := c.PostForm("content")

	if content == "" {
		jsonErr(c, 400, "Note content is required", nil)
		return
	}
	if len(content) > 2000 {
		jsonErr(c, 400, "Note content too long (max 2000 chars)", nil)
		return
	}

	_, err := h.orderSvc.AddNote(c.Request.Context(), order.ID, noteType, content, user.ID)
	if err != nil {
		jsonErr(c, 500, "Failed to add note", nil)
		return
	}

	h.setFlash(c, "success", "Note added")
	c.Redirect(http.StatusFound, fmt.Sprintf("/orders/%d", order.ID))
}

func (h *Handlers) OrderTimelineAPI(c *gin.Context) {
	order := h.loadAndAuthorizeOrder(c)
	if order == nil {
		return
	}
	history, _ := h.orderSvc.ListHistory(c.Request.Context(), order.ID)
	jsonOK(c, "ok", history)
}

func (h *Handlers) SplitOrderAPI(c *gin.Context) {
	order := h.loadAndAuthorizeOrder(c)
	if order == nil {
		return
	}
	user := getUser(c)

	var lineIDs []int
	lineIDStrs := c.PostFormArray("line_ids")
	for _, s := range lineIDStrs {
		if n, err := strconv.Atoi(s); err == nil {
			lineIDs = append(lineIDs, n)
		}
	}

	childID, err := h.orderSvc.SplitOrder(c.Request.Context(), order.ID, user.ID, lineIDs)
	if err != nil {
		h.setFlash(c, "error", "Split failed: "+err.Error())
		c.Redirect(http.StatusFound, fmt.Sprintf("/orders/%d", order.ID))
		return
	}

	h.setFlash(c, "success", "Order split successfully")
	c.Redirect(http.StatusFound, fmt.Sprintf("/orders/%d", childID))
}

// ====== Notifications ======

func (h *Handlers) NotificationsPage(c *gin.Context) {
	user := getUser(c)
	ctx := c.Request.Context()
	perms := middleware.GetPermissions(c.Request.Context())
	tab := c.DefaultQuery("tab", "inbox")
	canManageExports := perms["notification.manage"]

	// Server-side gate: reject export-queue tab for users without notification.manage.
	// Fall back to inbox rather than returning forbidden, to keep the page navigable.
	if tab == "export-queue" && !canManageExports {
		tab = "inbox"
	}

	pd := h.templData(c, "Notifications", "notifications")

	var nv []templviews.NotificationView
	var av []templviews.AnnouncementView
	var pv []templviews.PreferenceView
	var eq []templviews.ExportQueueView
	prefTypes := []string{"order_created", "order_transition", "alert_opened", "alert_closed", "catalog_published"}

	if h.notifSvc != nil {
		switch tab {
		case "inbox", "":
			notifs, _ := h.notifSvc.ListForUser(ctx, user.ID)
			for _, n := range notifs {
				nv = append(nv, templviews.NotificationView{
					ID: n.ID, Type: n.Type, Title: n.Title, Body: n.Body,
					IsRead: n.IsRead, CreatedAt: n.CreatedAt.Format("2006-01-02 15:04"),
				})
			}
		case "announcements":
			anns, _ := h.notifSvc.ListAnnouncements(ctx, user.ID)
			for _, a := range anns {
				av = append(av, templviews.AnnouncementView{
					ID: a.ID, Title: a.Title, Body: a.Body, Priority: a.Priority, IsRead: a.IsRead,
					CreatedAt: a.CreatedAt.Format("2006-01-02 15:04"),
				})
			}
		case "preferences":
			prefs, _ := h.notifSvc.GetPreferences(ctx, user.ID)
			for _, p := range prefs {
				pv = append(pv, templviews.PreferenceView{
					Channel: p.Channel, EventType: p.EventType, Enabled: p.Enabled,
				})
			}
		case "export-queue":
			queue, _ := h.notifSvc.ListExportQueue(ctx)
			for _, q := range queue {
				eq = append(eq, templviews.ExportQueueView{
					ID: q.ID, Channel: q.Channel, Recipient: q.Recipient, Status: q.Status,
					Attempts: q.Attempts, MaxAttempts: q.MaxAttempts, CreatedAt: q.CreatedAt.Format("2006-01-02 15:04"),
				})
			}
		}
	}

	views.RenderTempl(c, http.StatusOK, templviews.NotificationsPage(pd, tab, nv, av, pv, prefTypes, eq, canManageExports))
}

func (h *Handlers) ListNotificationsAPI(c *gin.Context) {
	user := getUser(c)
	notifs, _ := h.notifSvc.ListForUser(c.Request.Context(), user.ID)
	jsonOK(c, "ok", notifs)
}

func (h *Handlers) MarkNotificationReadAPI(c *gin.Context) {
	user := getUser(c)
	id := getIntParam(c, "id")
	h.notifSvc.MarkRead(c.Request.Context(), id, user.ID)
	jsonOK(c, "Marked as read", nil)
}

func (h *Handlers) BulkMarkReadAPI(c *gin.Context) {
	user := getUser(c)
	h.notifSvc.BulkMarkRead(c.Request.Context(), user.ID)
	h.setFlash(c, "success", "All notifications marked as read")
	c.Redirect(http.StatusFound, "/notifications")
}

func (h *Handlers) ListAnnouncementsAPI(c *gin.Context) {
	user := getUser(c)
	anns, _ := h.notifSvc.ListAnnouncements(c.Request.Context(), user.ID)
	jsonOK(c, "ok", anns)
}

func (h *Handlers) MarkAnnouncementReadAPI(c *gin.Context) {
	user := getUser(c)
	id := getIntParam(c, "id")
	if h.notifSvc != nil {
		h.notifSvc.MarkAnnouncementRead(c.Request.Context(), id, user.ID)
	}
	jsonOK(c, "Marked as read", nil)
}

func (h *Handlers) GetPreferencesAPI(c *gin.Context) {
	user := getUser(c)
	prefs, _ := h.notifSvc.GetPreferences(c.Request.Context(), user.ID)
	jsonOK(c, "ok", prefs)
}

func (h *Handlers) UpdatePreferencesAPI(c *gin.Context) {
	user := getUser(c)

	// Try JSON binding first
	var prefs []notifications.Preference
	if err := c.ShouldBindJSON(&prefs); err != nil {
		// Fall back to form data: parse checkbox fields like "in_app_order_created", "email_alert_opened"
		prefs = nil
		eventTypes := []string{"order_created", "order_transition", "alert_opened", "alert_closed", "catalog_published"}
		channels := []string{"in_app", "email", "sms", "webhook"}
		for _, et := range eventTypes {
			for _, ch := range channels {
				fieldName := ch + "_" + et
				if c.PostForm(fieldName) != "" {
					prefs = append(prefs, notifications.Preference{
						Channel:   ch,
						EventType: et,
						Enabled:   true,
					})
				}
			}
		}
	}

	// Guard: never delete all preferences on empty/malformed input
	if len(prefs) == 0 {
		h.setFlash(c, "error", "No valid preferences provided")
		c.Redirect(http.StatusFound, "/notifications?tab=preferences")
		return
	}

	if err := h.notifSvc.UpdatePreferences(c.Request.Context(), user.ID, prefs); err != nil {
		h.setFlash(c, "error", "Failed to save preferences: "+err.Error())
	} else {
		h.setFlash(c, "success", "Preferences updated")
	}
	c.Redirect(http.StatusFound, "/notifications?tab=preferences")
}

func (h *Handlers) ListExportQueueAPI(c *gin.Context) {
	queue, _ := h.notifSvc.ListExportQueue(c.Request.Context())
	jsonOK(c, "ok", queue)
}

// ====== Alerts ======

func (h *Handlers) AlertsPage(c *gin.Context) {
	status := c.Query("status")
	alertList, _ := h.alertSvc.ListAlerts(c.Request.Context(), status)

	pd := h.templData(c, "Alerts", "alerts")
	var av []templviews.AlertView
	for _, a := range alertList {
		av = append(av, templviews.AlertView{
			ID: a.ID, Title: a.Title, Severity: a.Severity, EntityType: a.EntityType,
			EntityID: a.EntityID, Status: a.Status, ClaimedByName: a.ClaimedByName,
		})
	}
	views.RenderTempl(c, http.StatusOK, templviews.AlertsPage(pd, av, status))
}

func (h *Handlers) ListAlertsAPI(c *gin.Context) {
	alertList, _ := h.alertSvc.ListAlerts(c.Request.Context(), c.Query("status"))
	jsonOK(c, "ok", alertList)
}

func (h *Handlers) ClaimAlertAPI(c *gin.Context) {
	user := getUser(c)
	id := getIntParam(c, "id")
	if err := h.alertSvc.ClaimAlert(c.Request.Context(), id, user.ID); err != nil {
		h.setFlash(c, "error", err.Error())
	} else {
		h.setFlash(c, "success", "Alert claimed")
	}
	c.Redirect(http.StatusFound, "/alerts")
}

func (h *Handlers) ProcessAlertAPI(c *gin.Context) {
	user := getUser(c)
	id := getIntParam(c, "id")
	if err := h.alertSvc.ProcessAlert(c.Request.Context(), id, user.ID); err != nil {
		h.setFlash(c, "error", err.Error())
	} else {
		h.setFlash(c, "success", "Alert now processing")
	}
	c.Redirect(http.StatusFound, "/alerts")
}

func (h *Handlers) CloseAlertAPI(c *gin.Context) {
	user := getUser(c)
	id := getIntParam(c, "id")
	notes := c.PostForm("resolution_notes")
	if err := h.alertSvc.CloseAlert(c.Request.Context(), id, user.ID, notes); err != nil {
		h.setFlash(c, "error", err.Error())
	} else {
		h.setFlash(c, "success", "Alert closed")
	}
	c.Redirect(http.StatusFound, "/alerts")
}

func (h *Handlers) EvaluateAlertsAPI(c *gin.Context) {
	count, err := h.alertSvc.EvaluateAlerts(c.Request.Context())
	if err != nil {
		jsonErr(c, 500, "Evaluation failed", nil)
		return
	}
	h.setFlash(c, "success", fmt.Sprintf("Alert evaluation complete: %d new alerts", count))
	c.Redirect(http.StatusFound, "/alerts")
}

func (h *Handlers) CloseAlertModal(c *gin.Context) {
	id := getIntParam(c, "id")
	data := h.baseData(c, "Close Alert", "alerts")
	data["AlertID"] = id
	h.renderer.HTMLFragment(c, http.StatusOK, "close-alert-modal", data)
}

// ====== Metrics ======

func (h *Handlers) MetricsPage(c *gin.Context) {
	metricList, _ := h.metricSvc.ListMetrics(c.Request.Context(), getUser(c).ID)
	data := h.baseData(c, "Metrics", "metrics")
	data["Metrics"] = metricList
	h.renderer.HTML(c, http.StatusOK, "metrics", data)
}

func (h *Handlers) MetricDetailPage(c *gin.Context) {
	id := getIntParam(c, "id")
	user := getUser(c)
	metric, err := h.metricSvc.GetMetric(c.Request.Context(), id)
	if err != nil {
		c.Redirect(http.StatusFound, "/metrics")
		return
	}
	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), id, user.ID, false); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied to this metric", nil)
		return
	}

	versions, _ := h.metricSvc.ListVersions(c.Request.Context(), id)
	dims, _ := h.metricSvc.ListDimensions(c.Request.Context(), id)
	lineage, _ := h.metricSvc.GetLineage(c.Request.Context(), id)
	impact, _ := h.metricSvc.GetLatestImpactAnalysis(c.Request.Context(), id)

	data := h.baseData(c, metric.Name, "metrics")
	data["Metric"] = metric
	data["Versions"] = versions
	data["Dimensions"] = dims
	data["Lineage"] = lineage
	data["ImpactAnalysis"] = impact
	data["ImpactAnalysisOK"] = impact != nil && impact.Status == "approved"
	h.renderer.HTML(c, http.StatusOK, "metric_detail", data)
}

func (h *Handlers) ListMetricsAPI(c *gin.Context) {
	metricList, _ := h.metricSvc.ListMetrics(c.Request.Context(), getUser(c).ID)
	jsonOK(c, "ok", metricList)
}

func (h *Handlers) CreateMetricAPI(c *gin.Context) {
	user := getUser(c)

	var params struct {
		Name              string                   `json:"name"`
		Description       string                   `json:"description"`
		SQLExpression     string                   `json:"sql_expression"`
		SemanticFormula   string                   `json:"semantic_formula"`
		TimeGrain         string                   `json:"time_grain"`
		IsDerived         bool                     `json:"is_derived"`
		WindowCalculation string                   `json:"window_calculation"`
		DependsOnMetrics  []int                    `json:"depends_on_metrics"`
		Filters           []metrics.MetricFilterDef `json:"filters"`
		Dimensions        []metrics.DimensionDef   `json:"dimensions"`
	}

	// Support both JSON and form-encoded requests
	formName := c.PostForm("name")
	if formName != "" {
		params.Name = formName
		params.Description = c.PostForm("description")
		params.SQLExpression = c.PostForm("sql_expression")
		params.SemanticFormula = c.PostForm("semantic_formula")
		params.TimeGrain = c.PostForm("time_grain")
		params.IsDerived = c.PostForm("is_derived") == "true"
		params.WindowCalculation = c.PostForm("window_calculation")
	} else {
		if err := c.ShouldBindJSON(&params); err != nil {
			jsonErr(c, 400, "Name is required", gin.H{"name": "Name is required"})
			return
		}
	}

	if params.Name == "" {
		jsonErr(c, 400, "Validation failed", gin.H{"name": "Name is required"})
		return
	}

	id, err := h.metricSvc.CreateMetric(c.Request.Context(), metrics.CreateParams{
		Name: params.Name, Description: params.Description, SQLExpression: params.SQLExpression,
		SemanticFormula: params.SemanticFormula, TimeGrain: params.TimeGrain,
		IsDerived: params.IsDerived, WindowCalculation: params.WindowCalculation,
		OwnerID: user.ID, DependsOnMetrics: params.DependsOnMetrics,
		Filters: params.Filters, Dimensions: params.Dimensions,
	})
	if err != nil {
		h.setFlash(c, "error", "Failed: "+err.Error())
		c.Redirect(http.StatusFound, "/metrics")
		return
	}

	h.setFlash(c, "success", "Metric created successfully")
	c.Redirect(http.StatusFound, fmt.Sprintf("/metrics/%d", id))
}

func (h *Handlers) UpdateMetricAPI(c *gin.Context) {
	user := getUser(c)
	id := getIntParam(c, "id")

	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), id, user.ID, false); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied", nil)
		return
	}

	var params struct {
		Description       string                   `json:"description"`
		SQLExpression     string                   `json:"sql_expression"`
		SemanticFormula   string                   `json:"semantic_formula"`
		TimeGrain         string                   `json:"time_grain"`
		IsDerived         bool                     `json:"is_derived"`
		WindowCalculation string                   `json:"window_calculation"`
		DependsOnMetrics  []int                    `json:"depends_on_metrics"`
		Filters           []metrics.MetricFilterDef `json:"filters"`
		Dimensions        []metrics.DimensionDef   `json:"dimensions"`
	}
	if err := c.ShouldBindJSON(&params); err != nil {
		jsonErr(c, 400, "Invalid request body", nil)
		return
	}

	if err := h.metricSvc.UpdateMetric(c.Request.Context(), metrics.UpdateParams{
		MetricID: id, Description: params.Description, SQLExpression: params.SQLExpression,
		SemanticFormula: params.SemanticFormula, TimeGrain: params.TimeGrain,
		IsDerived: params.IsDerived, WindowCalculation: params.WindowCalculation,
		ActorID: user.ID, DependsOnMetrics: params.DependsOnMetrics,
		Filters: params.Filters, Dimensions: params.Dimensions,
	}); err != nil {
		jsonErr(c, 500, "Update failed: "+err.Error(), nil)
		return
	}
	jsonOK(c, "Metric updated", nil)
}

func (h *Handlers) GetMetricAPI(c *gin.Context) {
	id := getIntParam(c, "id")
	user := getUser(c)
	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), id, user.ID, false); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied", nil)
		return
	}

	ctx := c.Request.Context()
	metric, err := h.metricSvc.GetMetric(ctx, id)
	if err != nil {
		jsonErr(c, 404, "Metric not found", nil)
		return
	}

	versions, _ := h.metricSvc.ListVersions(ctx, id)
	dimensions, _ := h.metricSvc.ListDimensions(ctx, id)
	filters, _ := h.metricSvc.ListFilters(ctx, id)
	dependencies, _ := h.metricSvc.ListDependencies(ctx, id)

	jsonOK(c, "ok", gin.H{
		"metric":       metric,
		"versions":     versions,
		"dimensions":   dimensions,
		"filters":      filters,
		"dependencies": dependencies,
	})
}

func (h *Handlers) ListMetricVersionsAPI(c *gin.Context) {
	id := getIntParam(c, "id")
	versions, err := h.metricSvc.ListVersions(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, 500, "Failed to list versions", nil)
		return
	}
	jsonOK(c, "ok", versions)
}

func (h *Handlers) ListMetricDimensionsAPI(c *gin.Context) {
	id := getIntParam(c, "id")
	dims, err := h.metricSvc.ListDimensions(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, 500, "Failed to list dimensions", nil)
		return
	}
	jsonOK(c, "ok", dims)
}

func (h *Handlers) ListMetricFiltersAPI(c *gin.Context) {
	id := getIntParam(c, "id")
	filters, err := h.metricSvc.ListFilters(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, 500, "Failed to list filters", nil)
		return
	}
	jsonOK(c, "ok", filters)
}

func (h *Handlers) ListMetricDependenciesAPI(c *gin.Context) {
	id := getIntParam(c, "id")
	deps, err := h.metricSvc.ListDependencies(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, 500, "Failed to list dependencies", nil)
		return
	}
	jsonOK(c, "ok", deps)
}

func (h *Handlers) AddMetricDimensionAPI(c *gin.Context) {
	user := getUser(c)
	metricID := getIntParam(c, "id")
	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), metricID, user.ID, false); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied", nil)
		return
	}
	var p struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		jsonErr(c, 400, "Invalid request body", nil)
		return
	}
	id, err := h.metricSvc.AddDimension(c.Request.Context(), metricID, metrics.DimensionDef{Name: p.Name, Description: p.Description}, user.ID)
	if err != nil {
		jsonErr(c, 400, err.Error(), nil)
		return
	}
	jsonOK(c, "Dimension added", gin.H{"id": id})
}

func (h *Handlers) RemoveMetricDimensionAPI(c *gin.Context) {
	user := getUser(c)
	metricID := getIntParam(c, "id")
	dimID := getIntParam(c, "dim_id")
	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), metricID, user.ID, false); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied", nil)
		return
	}
	if err := h.metricSvc.RemoveDimension(c.Request.Context(), metricID, dimID, user.ID); err != nil {
		jsonErr(c, 400, err.Error(), nil)
		return
	}
	jsonOK(c, "Dimension removed", nil)
}

func (h *Handlers) AddMetricFilterAPI(c *gin.Context) {
	user := getUser(c)
	metricID := getIntParam(c, "id")
	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), metricID, user.ID, false); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied", nil)
		return
	}
	var p struct {
		Name       string `json:"name"`
		Expression string `json:"expression"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		jsonErr(c, 400, "Invalid request body", nil)
		return
	}
	id, err := h.metricSvc.AddFilter(c.Request.Context(), metricID, metrics.MetricFilterDef{Name: p.Name, Expression: p.Expression}, user.ID)
	if err != nil {
		jsonErr(c, 400, err.Error(), nil)
		return
	}
	jsonOK(c, "Filter added", gin.H{"id": id})
}

func (h *Handlers) RemoveMetricFilterAPI(c *gin.Context) {
	user := getUser(c)
	metricID := getIntParam(c, "id")
	filterID := getIntParam(c, "filter_id")
	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), metricID, user.ID, false); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied", nil)
		return
	}
	if err := h.metricSvc.RemoveFilter(c.Request.Context(), metricID, filterID, user.ID); err != nil {
		jsonErr(c, 400, err.Error(), nil)
		return
	}
	jsonOK(c, "Filter removed", nil)
}

func (h *Handlers) AddMetricDependencyAPI(c *gin.Context) {
	user := getUser(c)
	metricID := getIntParam(c, "id")
	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), metricID, user.ID, false); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied", nil)
		return
	}
	var p struct {
		DependsOnMetricID int `json:"depends_on_metric_id"`
	}
	if err := c.ShouldBindJSON(&p); err != nil {
		jsonErr(c, 400, "Invalid request body", nil)
		return
	}
	id, err := h.metricSvc.AddDependency(c.Request.Context(), metricID, p.DependsOnMetricID, user.ID)
	if err != nil {
		jsonErr(c, 400, err.Error(), nil)
		return
	}
	jsonOK(c, "Dependency added", gin.H{"id": id})
}

func (h *Handlers) RemoveMetricDependencyAPI(c *gin.Context) {
	user := getUser(c)
	metricID := getIntParam(c, "id")
	depID := getIntParam(c, "dep_id")
	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), metricID, user.ID, false); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied", nil)
		return
	}
	if err := h.metricSvc.RemoveDependency(c.Request.Context(), metricID, depID, user.ID); err != nil {
		jsonErr(c, 400, err.Error(), nil)
		return
	}
	jsonOK(c, "Dependency removed", nil)
}

func (h *Handlers) ImpactAnalysisAPI(c *gin.Context) {
	id := getIntParam(c, "id")
	user := getUser(c)
	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), id, user.ID, true); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied", nil)
		return
	}
	analysis, err := h.metricSvc.RunImpactAnalysis(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, 500, "Analysis failed", nil)
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		html := fmt.Sprintf(`<div class="modal-overlay" onclick="if(event.target===this)this.remove()"><div class="modal"><div class="modal-header"><h3>Impact Analysis</h3><button onclick="this.closest('.modal-overlay').remove()" class="modal-close">&times;</button></div><div class="modal-body"><p>Dependent Metrics: %d</p><p>Dependent Charts: %d</p><p>Missing Dependencies: %d</p><p>Status: <span class="badge badge-%s">%s</span></p></div></div></div>`,
			analysis.DependentMetrics, analysis.DependentCharts, analysis.MissingDeps, analysis.Status, analysis.Status)
		c.Data(200, "text/html", []byte(html))
		return
	}
	jsonOK(c, "Analysis complete", analysis)
}

func (h *Handlers) ActivateMetricAPI(c *gin.Context) {
	user := getUser(c)
	id := getIntParam(c, "id")
	if err := h.metricSvc.ActivateMetric(c.Request.Context(), id, user.ID); err != nil {
		h.setFlash(c, "error", err.Error())
	} else {
		h.setFlash(c, "success", "Metric activated")
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/metrics/%d", id))
}

func (h *Handlers) MetricLineageAPI(c *gin.Context) {
	id := getIntParam(c, "id")
	user := getUser(c)
	if err := h.metricSvc.CheckMetricPermission(c.Request.Context(), id, user.ID, false); err != nil {
		jsonErr(c, http.StatusForbidden, "Access denied to this metric", nil)
		return
	}
	lineage, err := h.metricSvc.GetLineage(c.Request.Context(), id)
	if err != nil {
		jsonErr(c, 500, "Failed to get lineage", nil)
		return
	}
	jsonOK(c, "ok", lineage)
}

// ====== Audit ======

func (h *Handlers) AuditPage(c *gin.Context) {
	entityType := c.Query("entity_type")
	entityID := getIntQuery(c, "entity_id", 0)
	page := getIntQuery(c, "page", 1)

	entries, total, _ := h.auditSvc.List(c.Request.Context(), audit.ListParams{
		EntityType: entityType, EntityID: entityID, Limit: 50, Offset: (page - 1) * 50,
	})
	totalPages := int(math.Ceil(float64(total) / 50))

	data := h.baseData(c, "Audit Log", "audit")
	data["Entries"] = entries
	data["FilterEntityType"] = entityType
	data["FilterEntityID"] = entityID
	data["Pagination"] = Pagination{Page: page, TotalPages: totalPages, Total: total}
	h.renderer.HTML(c, http.StatusOK, "audit", data)
}

func (h *Handlers) ListAuditAPI(c *gin.Context) {
	entityType := c.Query("entity_type")
	entityID := getIntQuery(c, "entity_id", 0)
	entries, total, _ := h.auditSvc.List(c.Request.Context(), audit.ListParams{
		EntityType: entityType, EntityID: entityID, Limit: 50,
	})
	jsonOK(c, "ok", gin.H{"entries": entries, "total": total})
}

func (h *Handlers) AuditByEntityAPI(c *gin.Context) {
	entityType := c.Param("entity_type")
	entityID := getIntParam(c, "entity_id")
	entries, total, _ := h.auditSvc.List(c.Request.Context(), audit.ListParams{
		EntityType: entityType, EntityID: entityID, Limit: 100,
	})
	jsonOK(c, "ok", gin.H{"entries": entries, "total": total})
}

// ====== Missing modal handlers (wired to routes) ======

func (h *Handlers) AddCartItemModal(c *gin.Context) {
	cartID := getIntParam(c, "id")
	models, _, _ := h.catalogSvc.ListModels(c.Request.Context(), catalog.ListParams{Status: "published", PageSize: 100})
	csrf := middleware.GetCSRFToken(c)
	html := fmt.Sprintf(`<div class="modal-overlay" onclick="if(event.target===this)this.remove()"><div class="modal"><div class="modal-header"><h3>Add Item to Cart</h3><button onclick="this.closest('.modal-overlay').remove()" class="modal-close">&times;</button></div><div class="modal-body"><form method="POST" action="/api/carts/%d/items"><input type="hidden" name="csrf_token" value="%s"><div class="form-group"><label>Vehicle</label><select name="vehicle_model_id" class="form-control" required>`, cartID, csrf)
	for _, m := range models {
		html += fmt.Sprintf(`<option value="%d">%s (%s) - Stock: %d</option>`, m.ID, m.ModelName, m.ModelCode, m.StockQuantity)
	}
	html += `</select></div><div class="form-group"><label>Quantity</label><input type="number" name="quantity" value="1" min="1" class="form-control" required></div><button type="submit" class="btn btn-primary">Add</button></form></div></div></div>`
	c.Data(http.StatusOK, "text/html", []byte(html))
}

func (h *Handlers) SplitOrderModal(c *gin.Context) {
	id := getIntParam(c, "id")
	lines, _ := h.orderSvc.ListOrderLines(c.Request.Context(), id)
	csrf := middleware.GetCSRFToken(c)
	html := fmt.Sprintf(`<div class="modal-overlay" onclick="if(event.target===this)this.remove()"><div class="modal"><div class="modal-header"><h3>Split Order</h3><button onclick="this.closest('.modal-overlay').remove()" class="modal-close">&times;</button></div><div class="modal-body"><p>Select lines to move to a new backorder:</p><form method="POST" action="/api/orders/%d/split"><input type="hidden" name="csrf_token" value="%s">`, id, csrf)
	for _, l := range lines {
		html += fmt.Sprintf(`<label class="form-group"><input type="checkbox" name="line_ids" value="%d"> %s (Req: %d, Alloc: %d, Backorder: %d)</label><br>`, l.ID, l.VehicleName, l.QuantityRequested, l.QuantityAllocated, l.QuantityBackordered)
	}
	html += `<button type="submit" class="btn btn-primary" style="margin-top:1rem">Split Selected</button></form></div></div></div>`
	c.Data(http.StatusOK, "text/html", []byte(html))
}

func (h *Handlers) CreateMetricModal(c *gin.Context) {
	csrf := middleware.GetCSRFToken(c)
	html := fmt.Sprintf(`<div class="modal-overlay" onclick="if(event.target===this)this.remove()"><div class="modal"><div class="modal-header"><h3>New Metric Definition</h3><button onclick="this.closest('.modal-overlay').remove()" class="modal-close">&times;</button></div><div class="modal-body"><form method="POST" action="/api/metrics" id="new-metric-form"><input type="hidden" name="csrf_token" value="%s"><div class="form-group"><label>Name</label><input type="text" name="name" class="form-control" required></div><div class="form-group"><label>Description</label><textarea name="description" class="form-control" rows="2"></textarea></div><div class="form-group"><label>SQL Expression</label><textarea name="sql_expression" class="form-control" rows="3"></textarea></div><div class="form-group"><label>Time Grain</label><select name="time_grain" class="form-control"><option value="daily">Daily</option><option value="weekly">Weekly</option><option value="monthly">Monthly</option></select></div><button type="submit" class="btn btn-primary">Create</button></form></div></div></div>`, csrf)
	c.Data(http.StatusOK, "text/html", []byte(html))
}

// ====== Unused import suppressor ======
var _ = json.Marshal
var _ = slog.Info
var _ = time.Now

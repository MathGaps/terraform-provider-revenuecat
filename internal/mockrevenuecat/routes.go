package mockrevenuecat

import (
	"net/http"
)

// routeProjects dispatches everything under /v2/projects.
//
// segments is the path after "projects", so:
//
//	[]                                    -> list projects
//	[projectID]                           -> read one project
//	[projectID, "apps"]                   -> collection
//	[projectID, "apps", appID]            -> item
//	[projectID, "entitlements", id, "actions", "attach_products"]
func (s *Server) routeProjects(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 0 {
		if r.Method != http.MethodGet {
			s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "projects are read-only")
			return
		}
		s.listEnvelope(w, r, "/v2/projects", s.renderAll("project", nil))
		return
	}

	projectID := segments[0]

	if len(segments) == 1 {
		if r.Method != http.MethodGet {
			s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "projects are read-only")
			return
		}
		project, ok := s.store.get("project", "", projectID)
		if !ok {
			s.notFound(w, "project")
			return
		}
		s.writeJSON(w, http.StatusOK, renderProject(project))
		return
	}

	// Every collection below lives inside a project, so an unknown project is
	// reported before anything else is considered.
	if _, ok := s.store.get("project", "", projectID); !ok {
		s.notFound(w, "project")
		return
	}

	collection := segments[1]
	rest := segments[2:]

	switch collection {
	case "apps":
		s.routeCollection(w, r, collectionSpec{
			kind:           "app",
			projectID:      projectID,
			basePath:       "/v2/projects/" + projectID + "/apps",
			render:         renderApp,
			create:         appFields,
			update:         appUpdateFields,
			validateCreate: validateAppCreate,
		}, rest)

	case "products":
		s.routeCollection(w, r, collectionSpec{
			kind:      "product",
			projectID: projectID,
			basePath:  "/v2/projects/" + projectID + "/products",
			render:    renderProduct,
			create:    productFields,
			update:    productUpdateFields,
			validateCreate: func(body map[string]any) *validationError {
				return s.validateProductCreate(projectID, body)
			},
			validateUpdate: validateProductUpdate,
		}, rest)

	case "entitlements":
		s.routeEntitlements(w, r, projectID, rest)

	case "offerings":
		s.routeOfferings(w, r, projectID, rest)

	case "packages":
		s.routePackages(w, r, projectID, rest)

	case "integrations":
		// Webhooks are the one integration served: /integrations/webhooks[/<id>].
		if len(rest) == 0 || rest[0] != "webhooks" {
			s.notFound(w, "integration")
			return
		}
		s.routeCollection(w, r, collectionSpec{
			kind:      "webhook_integration",
			projectID: projectID,
			basePath:  "/v2/projects/" + projectID + "/integrations/webhooks",
			render:    renderWebhookIntegration,
			create:    webhookIntegrationFields,
			update:    webhookIntegrationFields,
			validateCreate: func(body map[string]any) *validationError {
				if verr := validateWebhookIntegrationCreate(body); verr != nil {
					return verr
				}
				return s.validateAppInProject(projectID, body)
			},
			validateUpdate: func(body map[string]any) *validationError {
				return s.validateAppInProject(projectID, body)
			},
		}, rest[1:])

	default:
		s.notFound(w, "collection "+collection)
	}
}

// collectionSpec describes a plain CRUD collection.
type collectionSpec struct {
	kind      string
	projectID string
	basePath  string

	// render turns a stored object into its API representation.
	render func(*object) map[string]any

	// create maps a create request body onto stored fields.
	create func(body map[string]any) map[string]any

	// update maps an update request body onto changed fields. A nil update
	// means the collection does not support updates.
	update func(body map[string]any) map[string]any

	// validateCreate rejects a create body the real API would 400 on, before
	// create ever runs. copyFields-based create funcs silently drop anything
	// they don't recognize, so without this a body missing a required nested
	// object (app) or carrying one the API disallows (offering's is_current
	// on create) would succeed here and only fail against the real service —
	// which is exactly how both of those bugs shipped once already.
	validateCreate func(body map[string]any) *validationError

	// validateUpdate is validateCreate's counterpart for the update body —
	// several resources accept a different, narrower field set on update than
	// on create (an entitlement's or package's lookup_key, for instance).
	validateUpdate func(body map[string]any) *validationError

	// match optionally narrows which stored objects belong to this collection.
	match func(*object) bool

	// afterWrite runs after a create or an update is stored, for a write that
	// changes other objects too (making one offering current clears the rest).
	afterWrite func(*object)
}

// validationError is an error the mock returns instead of running a create or
// an update, for a request the real API rejects: a 400 unless status says
// otherwise.
type validationError struct {
	status  int
	code    string
	message string
}

func (v *validationError) httpStatus() int {
	if v.status != 0 {
		return v.status
	}
	return http.StatusBadRequest
}

func (s *Server) routeCollection(w http.ResponseWriter, r *http.Request, spec collectionSpec, segments []string) {
	switch {
	case len(segments) == 0:
		switch r.Method {
		case http.MethodGet:
			s.listEnvelope(w, r, spec.basePath, s.renderAllScoped(spec))
		case http.MethodPost:
			body, err := decode(r)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
				return
			}
			if spec.validateCreate != nil {
				if verr := spec.validateCreate(body); verr != nil {
					s.writeError(w, verr.httpStatus(), verr.code, verr.message)
					return
				}
			}
			created := s.store.create(spec.kind, spec.projectID, spec.create(body))
			if spec.afterWrite != nil {
				spec.afterWrite(created)
			}
			s.writeJSON(w, http.StatusCreated, spec.render(created))
		default:
			s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method+" is not supported here")
		}

	case len(segments) == 1:
		id := segments[0]
		switch r.Method {
		case http.MethodGet:
			obj, ok := s.store.get(spec.kind, spec.projectID, id)
			if !ok {
				s.notFound(w, spec.kind)
				return
			}
			s.writeJSON(w, http.StatusOK, spec.render(obj))

		case http.MethodPost:
			if spec.update == nil {
				s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", spec.kind+" cannot be updated")
				return
			}
			body, err := decode(r)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
				return
			}
			if spec.validateUpdate != nil {
				if verr := spec.validateUpdate(body); verr != nil {
					s.writeError(w, verr.httpStatus(), verr.code, verr.message)
					return
				}
			}
			updated, ok := s.store.update(spec.kind, spec.projectID, id, spec.update(body))
			if !ok {
				s.notFound(w, spec.kind)
				return
			}
			if spec.afterWrite != nil {
				spec.afterWrite(updated)
			}
			s.writeJSON(w, http.StatusOK, spec.render(updated))

		case http.MethodDelete:
			if !s.store.delete(spec.kind, spec.projectID, id) {
				s.notFound(w, spec.kind)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method+" is not supported here")
		}

	default:
		s.notFound(w, "endpoint")
	}
}

func (s *Server) routeEntitlements(w http.ResponseWriter, r *http.Request, projectID string, segments []string) {
	spec := collectionSpec{
		kind:           "entitlement",
		projectID:      projectID,
		basePath:       "/v2/projects/" + projectID + "/entitlements",
		render:         renderEntitlement,
		create:         entitlementFields,
		update:         entitlementFields,
		validateCreate: validateEntitlementCreate,
		validateUpdate: validateEntitlementUpdate,
	}

	// /entitlements/<id>/products and /entitlements/<id>/actions/<action>
	if len(segments) >= 2 {
		s.routeAttachments(w, r, "entitlement", projectID, segments, renderProduct)
		return
	}

	s.routeCollection(w, r, spec, segments)
}

func (s *Server) routeOfferings(w http.ResponseWriter, r *http.Request, projectID string, segments []string) {
	spec := collectionSpec{
		kind:      "offering",
		projectID: projectID,
		basePath:  "/v2/projects/" + projectID + "/offerings",
		render:    renderOffering,
		// The real API makes a project's first offering its current one: the
		// create body cannot carry is_current, yet it answers true (observed
		// 2026-10-13, the first apply of MathGaps/iac stacks/revenuecat-dev).
		create: func(body map[string]any) map[string]any {
			fields := offeringFields(body)
			if !s.store.hasCurrentOffering(projectID) {
				fields["is_current"] = true
			}
			return fields
		},
		update:         offeringFields,
		validateCreate: validateOfferingCreate,
		validateUpdate: func(body map[string]any) *validationError {
			if current, ok := body["is_current"].(bool); ok && !current && s.options.ClearCurrentStatus != 0 {
				return &validationError{status: s.options.ClearCurrentStatus, code: "parameter_error", message: "a project keeps a current offering: make another offering current instead"}
			}
			return nil
		},
		// A project has one current offering: making one current clears the
		// flag on the others.
		afterWrite: func(obj *object) {
			if current, _ := obj.Fields["is_current"].(bool); current {
				s.store.clearOtherCurrentOfferings(projectID, obj.ID)
			}
		},
	}

	// /offerings/<id>/packages is the packages collection scoped to an offering.
	if len(segments) >= 2 && segments[1] == "packages" {
		offeringID := segments[0]
		if _, ok := s.store.get("offering", projectID, offeringID); !ok {
			s.notFound(w, "offering")
			return
		}

		packageSpec := collectionSpec{
			kind:      "package",
			projectID: projectID,
			basePath:  "/v2/projects/" + projectID + "/offerings/" + offeringID + "/packages",
			render:    renderPackage,
			create: func(body map[string]any) map[string]any {
				fields := packageFields(body)
				fields["offering_id"] = offeringID
				// The real create endpoint does not reliably honor a requested
				// position — observed returning 1 for a create request that sent
				// 2. Mirror that here (always 1 on create, regardless of what was
				// requested) so the provider's Create() is forced through the
				// same follow-up-update self-heal it needs against the real API.
				fields["position"] = float64(1)
				return fields
			},
			update:         packageFields,
			validateCreate: validatePackageCreate,
			validateUpdate: validatePackageUpdate,
			match: func(obj *object) bool {
				return obj.Fields["offering_id"] == offeringID
			},
		}
		s.routeCollection(w, r, packageSpec, segments[2:])
		return
	}

	s.routeCollection(w, r, spec, segments)
}

func (s *Server) routePackages(w http.ResponseWriter, r *http.Request, projectID string, segments []string) {
	// /packages/<id>/products and /packages/<id>/actions/<action>
	if len(segments) >= 2 {
		s.routeAttachments(w, r, "package", projectID, segments, renderProduct)
		return
	}

	s.routeCollection(w, r, collectionSpec{
		kind:           "package",
		projectID:      projectID,
		basePath:       "/v2/projects/" + projectID + "/packages",
		render:         renderPackage,
		create:         packageFields,
		update:         packageFields,
		validateCreate: validatePackageCreate,
		validateUpdate: validatePackageUpdate,
	}, segments)
}

// routeAttachments serves the attached-product listing and the attach and
// detach actions for an entitlement or a package.
func (s *Server) routeAttachments(
	w http.ResponseWriter,
	r *http.Request,
	kind, projectID string,
	segments []string,
	renderProductObject func(*object) map[string]any,
) {
	parentID := segments[0]
	if _, ok := s.store.get(kind, projectID, parentID); !ok {
		s.notFound(w, kind)
		return
	}

	switch {
	case len(segments) == 2 && segments[1] == "products":
		if r.Method != http.MethodGet {
			s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method+" is not supported here")
			return
		}

		items := []map[string]any{}
		for _, entry := range s.store.attached(kind, parentID) {
			product, ok := s.store.get("product", "", entry.ProductID)
			if !ok {
				// A product deleted while still attached is dropped rather
				// than reported, matching how a real service would stop
				// listing it.
				continue
			}

			rendered := renderProductObject(product)
			if kind == "package" {
				// A package's products carry the criteria they were attached
				// under, nested alongside the product.
				items = append(items, map[string]any{
					"product_id":           entry.ProductID,
					"eligibility_criteria": entry.EligibilityCriteria,
					"product":              rendered,
				})
				continue
			}
			items = append(items, rendered)
		}

		base := "/v2/projects/" + projectID + "/" + kind + "s/" + parentID + "/products"
		s.listEnvelope(w, r, base, items)

	case len(segments) == 3 && segments[1] == "actions":
		if r.Method != http.MethodPost {
			s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method+" is not supported here")
			return
		}

		body, err := decode(r)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}

		switch segments[2] {
		case "attach_products":
			if kind == "package" {
				if verr := validatePackageAttachProducts(body); verr != nil {
					s.writeError(w, verr.httpStatus(), verr.code, verr.message)
					return
				}
			}
			s.store.attach(kind, parentID, attachmentsFromBody(body))
		case "detach_products":
			s.store.detach(kind, parentID, productIDsFromBody(body))
		default:
			s.notFound(w, "action "+segments[2])
			return
		}

		s.writeJSON(w, http.StatusOK, map[string]any{"object": kind, "id": parentID})

	default:
		s.notFound(w, "endpoint")
	}
}

// renderAll renders every object of a kind.
func (s *Server) renderAll(kind string, match func(*object) bool) []map[string]any {
	objects := s.store.list(kind, match)

	items := make([]map[string]any, 0, len(objects))
	for _, obj := range objects {
		switch kind {
		case "project":
			items = append(items, renderProject(obj))
		default:
			items = append(items, renderGeneric(obj))
		}
	}
	return items
}

// renderAllScoped renders every object belonging to a collection.
func (s *Server) renderAllScoped(spec collectionSpec) []map[string]any {
	objects := s.store.list(spec.kind, func(obj *object) bool {
		if obj.ProjectID != spec.projectID {
			return false
		}
		if spec.match != nil {
			return spec.match(obj)
		}
		return true
	})

	items := make([]map[string]any, 0, len(objects))
	for _, obj := range objects {
		items = append(items, spec.render(obj))
	}
	return items
}

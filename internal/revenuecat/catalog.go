package revenuecat

import (
	"context"
	"fmt"
	"net/url"
)

// --- Projects -------------------------------------------------------------

// GetProject reads a single project.
func (c *Client) GetProject(ctx context.Context, projectID string) (*Project, error) {
	var out Project
	if err := c.do(ctx, "GET", fmt.Sprintf("/projects/%s", url.PathEscape(projectID)), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListProjects returns every project the API key can access.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	return listAll[Project](ctx, c, "/projects", nil)
}

// --- Apps -----------------------------------------------------------------

// CreateApp creates an app in a project.
func (c *Client) CreateApp(ctx context.Context, projectID string, req CreateAppRequest) (*App, error) {
	var out App
	if err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/apps", url.PathEscape(projectID)), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetApp reads a single app.
func (c *Client) GetApp(ctx context.Context, projectID, appID string) (*App, error) {
	var out App
	if err := c.do(ctx, "GET", fmt.Sprintf("/projects/%s/apps/%s", url.PathEscape(projectID), url.PathEscape(appID)), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateApp updates an app in place.
func (c *Client) UpdateApp(ctx context.Context, projectID, appID string, req UpdateAppRequest) (*App, error) {
	var out App
	if err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/apps/%s", url.PathEscape(projectID), url.PathEscape(appID)), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteApp removes an app.
func (c *Client) DeleteApp(ctx context.Context, projectID, appID string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/projects/%s/apps/%s", url.PathEscape(projectID), url.PathEscape(appID)), nil, nil, nil)
}

// ListApps returns every app in a project.
func (c *Client) ListApps(ctx context.Context, projectID string) ([]App, error) {
	return listAll[App](ctx, c, fmt.Sprintf("/projects/%s/apps", url.PathEscape(projectID)), nil)
}

// --- Products -------------------------------------------------------------

// CreateProduct creates a product in a project.
func (c *Client) CreateProduct(ctx context.Context, projectID string, req CreateProductRequest) (*Product, error) {
	var out Product
	if err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/products", url.PathEscape(projectID)), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetProduct reads a single product.
func (c *Client) GetProduct(ctx context.Context, projectID, productID string) (*Product, error) {
	var out Product
	if err := c.do(ctx, "GET", fmt.Sprintf("/projects/%s/products/%s", url.PathEscape(projectID), url.PathEscape(productID)), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateProduct changes a product's display name in place.
func (c *Client) UpdateProduct(ctx context.Context, projectID, productID string, req UpdateProductRequest) (*Product, error) {
	var out Product
	if err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/products/%s", url.PathEscape(projectID), url.PathEscape(productID)), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteProduct removes a product.
func (c *Client) DeleteProduct(ctx context.Context, projectID, productID string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/projects/%s/products/%s", url.PathEscape(projectID), url.PathEscape(productID)), nil, nil, nil)
}

// ListProducts returns every product in a project.
func (c *Client) ListProducts(ctx context.Context, projectID string) ([]Product, error) {
	return listAll[Product](ctx, c, fmt.Sprintf("/projects/%s/products", url.PathEscape(projectID)), nil)
}

// --- Entitlements ---------------------------------------------------------

// CreateEntitlement creates an entitlement in a project.
func (c *Client) CreateEntitlement(ctx context.Context, projectID string, req CreateEntitlementRequest) (*Entitlement, error) {
	var out Entitlement
	if err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/entitlements", url.PathEscape(projectID)), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetEntitlement reads a single entitlement.
func (c *Client) GetEntitlement(ctx context.Context, projectID, entitlementID string) (*Entitlement, error) {
	var out Entitlement
	if err := c.do(ctx, "GET", fmt.Sprintf("/projects/%s/entitlements/%s", url.PathEscape(projectID), url.PathEscape(entitlementID)), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateEntitlement updates an entitlement in place.
func (c *Client) UpdateEntitlement(ctx context.Context, projectID, entitlementID string, req UpdateEntitlementRequest) (*Entitlement, error) {
	var out Entitlement
	if err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/entitlements/%s", url.PathEscape(projectID), url.PathEscape(entitlementID)), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteEntitlement removes an entitlement.
func (c *Client) DeleteEntitlement(ctx context.Context, projectID, entitlementID string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/projects/%s/entitlements/%s", url.PathEscape(projectID), url.PathEscape(entitlementID)), nil, nil, nil)
}

// ListEntitlements returns every entitlement in a project.
func (c *Client) ListEntitlements(ctx context.Context, projectID string) ([]Entitlement, error) {
	return listAll[Entitlement](ctx, c, fmt.Sprintf("/projects/%s/entitlements", url.PathEscape(projectID)), nil)
}

// --- Offerings ------------------------------------------------------------

// CreateOffering creates an offering in a project.
func (c *Client) CreateOffering(ctx context.Context, projectID string, req CreateOfferingRequest) (*Offering, error) {
	var out Offering
	if err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/offerings", url.PathEscape(projectID)), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetOffering reads a single offering.
func (c *Client) GetOffering(ctx context.Context, projectID, offeringID string) (*Offering, error) {
	var out Offering
	if err := c.do(ctx, "GET", fmt.Sprintf("/projects/%s/offerings/%s", url.PathEscape(projectID), url.PathEscape(offeringID)), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateOffering updates an offering in place.
func (c *Client) UpdateOffering(ctx context.Context, projectID, offeringID string, req UpdateOfferingRequest) (*Offering, error) {
	var out Offering
	if err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/offerings/%s", url.PathEscape(projectID), url.PathEscape(offeringID)), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteOffering removes an offering.
func (c *Client) DeleteOffering(ctx context.Context, projectID, offeringID string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/projects/%s/offerings/%s", url.PathEscape(projectID), url.PathEscape(offeringID)), nil, nil, nil)
}

// ListOfferings returns every offering in a project.
func (c *Client) ListOfferings(ctx context.Context, projectID string) ([]Offering, error) {
	return listAll[Offering](ctx, c, fmt.Sprintf("/projects/%s/offerings", url.PathEscape(projectID)), nil)
}

// --- Packages -------------------------------------------------------------

// CreatePackage creates a package within an offering.
func (c *Client) CreatePackage(ctx context.Context, projectID, offeringID string, req CreatePackageRequest) (*Package, error) {
	var out Package
	path := fmt.Sprintf("/projects/%s/offerings/%s/packages", url.PathEscape(projectID), url.PathEscape(offeringID))
	if err := c.do(ctx, "POST", path, nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPackage reads a single package.
func (c *Client) GetPackage(ctx context.Context, projectID, packageID string) (*Package, error) {
	var out Package
	if err := c.do(ctx, "GET", fmt.Sprintf("/projects/%s/packages/%s", url.PathEscape(projectID), url.PathEscape(packageID)), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdatePackage updates a package in place.
func (c *Client) UpdatePackage(ctx context.Context, projectID, packageID string, req UpdatePackageRequest) (*Package, error) {
	var out Package
	if err := c.do(ctx, "POST", fmt.Sprintf("/projects/%s/packages/%s", url.PathEscape(projectID), url.PathEscape(packageID)), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeletePackage removes a package.
func (c *Client) DeletePackage(ctx context.Context, projectID, packageID string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/projects/%s/packages/%s", url.PathEscape(projectID), url.PathEscape(packageID)), nil, nil, nil)
}

// ListPackages returns every package in an offering.
func (c *Client) ListPackages(ctx context.Context, projectID, offeringID string) ([]Package, error) {
	path := fmt.Sprintf("/projects/%s/offerings/%s/packages", url.PathEscape(projectID), url.PathEscape(offeringID))
	return listAll[Package](ctx, c, path, nil)
}

// --- Attachments ----------------------------------------------------------

// AttachProductsToEntitlement attaches products to an entitlement. Attaching a
// product that is already attached is a no-op on RevenueCat's side.
func (c *Client) AttachProductsToEntitlement(ctx context.Context, projectID, entitlementID string, productIDs []string) error {
	if len(productIDs) == 0 {
		return nil
	}
	path := fmt.Sprintf("/projects/%s/entitlements/%s/actions/attach_products", url.PathEscape(projectID), url.PathEscape(entitlementID))
	return c.do(ctx, "POST", path, nil, attachProductsRequest{ProductIDs: productIDs}, nil)
}

// DetachProductsFromEntitlement detaches products from an entitlement.
func (c *Client) DetachProductsFromEntitlement(ctx context.Context, projectID, entitlementID string, productIDs []string) error {
	if len(productIDs) == 0 {
		return nil
	}
	path := fmt.Sprintf("/projects/%s/entitlements/%s/actions/detach_products", url.PathEscape(projectID), url.PathEscape(entitlementID))
	return c.do(ctx, "POST", path, nil, attachProductsRequest{ProductIDs: productIDs}, nil)
}

// ListEntitlementProducts returns the products currently attached to an
// entitlement.
func (c *Client) ListEntitlementProducts(ctx context.Context, projectID, entitlementID string) ([]Product, error) {
	path := fmt.Sprintf("/projects/%s/entitlements/%s/products", url.PathEscape(projectID), url.PathEscape(entitlementID))
	return listAll[Product](ctx, c, path, nil)
}

// AttachProductsToPackage attaches products to a package, each with its own
// eligibility criteria.
func (c *Client) AttachProductsToPackage(ctx context.Context, projectID, packageID string, products []PackageProduct) error {
	if len(products) == 0 {
		return nil
	}
	path := fmt.Sprintf("/projects/%s/packages/%s/actions/attach_products", url.PathEscape(projectID), url.PathEscape(packageID))
	return c.do(ctx, "POST", path, nil, attachPackageProductsRequest{Products: products}, nil)
}

// DetachProductsFromPackage detaches products from a package.
func (c *Client) DetachProductsFromPackage(ctx context.Context, projectID, packageID string, productIDs []string) error {
	if len(productIDs) == 0 {
		return nil
	}
	path := fmt.Sprintf("/projects/%s/packages/%s/actions/detach_products", url.PathEscape(projectID), url.PathEscape(packageID))
	return c.do(ctx, "POST", path, nil, attachProductsRequest{ProductIDs: productIDs}, nil)
}

// ListPackageProducts returns the products currently attached to a package,
// each with the eligibility criteria it was attached under.
func (c *Client) ListPackageProducts(ctx context.Context, projectID, packageID string) ([]PackageProduct, error) {
	path := fmt.Sprintf("/projects/%s/packages/%s/products", url.PathEscape(projectID), url.PathEscape(packageID))
	entries, err := listAll[packageProductEntry](ctx, c, path, nil)
	if err != nil {
		return nil, err
	}

	products := make([]PackageProduct, 0, len(entries))
	for _, entry := range entries {
		id := entry.ProductID
		if id == "" && entry.Product != nil {
			id = entry.Product.ID
		}
		if id == "" {
			continue
		}
		products = append(products, PackageProduct{
			ProductID:           id,
			EligibilityCriteria: entry.EligibilityCriteria,
		})
	}
	return products, nil
}

// --- Webhook integrations -------------------------------------------------

func webhookIntegrationsPath(projectID string) string {
	return fmt.Sprintf("/projects/%s/integrations/webhooks", url.PathEscape(projectID))
}

// CreateWebhookIntegration creates a webhook in a project.
func (c *Client) CreateWebhookIntegration(ctx context.Context, projectID string, req CreateWebhookIntegrationRequest) (*WebhookIntegration, error) {
	var out WebhookIntegration
	if err := c.do(ctx, "POST", webhookIntegrationsPath(projectID), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWebhookIntegration reads a single webhook.
func (c *Client) GetWebhookIntegration(ctx context.Context, projectID, webhookID string) (*WebhookIntegration, error) {
	var out WebhookIntegration
	if err := c.do(ctx, "GET", webhookIntegrationsPath(projectID)+"/"+url.PathEscape(webhookID), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateWebhookIntegration changes a webhook in place.
func (c *Client) UpdateWebhookIntegration(ctx context.Context, projectID, webhookID string, req UpdateWebhookIntegrationRequest) (*WebhookIntegration, error) {
	var out WebhookIntegration
	if err := c.do(ctx, "POST", webhookIntegrationsPath(projectID)+"/"+url.PathEscape(webhookID), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWebhookIntegration removes a webhook.
func (c *Client) DeleteWebhookIntegration(ctx context.Context, projectID, webhookID string) error {
	return c.do(ctx, "DELETE", webhookIntegrationsPath(projectID)+"/"+url.PathEscape(webhookID), nil, nil, nil)
}

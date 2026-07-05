# GSLB REST API

## Swagger UI


<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
<style>
  /* Ensure Swagger UI fits nicely and supports dark/light mode harmoniously */
  .swagger-ui {
    background-color: var(--md-card-background, #fff);
    border-radius: 4px;
    padding: 10px;
    margin-top: 15px;
  }
  .swagger-ui .info {
    margin: 20px 0 !important;
  }
  /* Adjust swagger colors for dark mode if user is in slate theme */
  [data-md-color-scheme="slate"] .swagger-ui {
    filter: invert(0.9) hue-rotate(180deg);
  }
  [data-md-color-scheme="slate"] .swagger-ui .microlight {
    filter: invert(1) hue-rotate(180deg);
  }
</style>

<div id="swagger-ui-container"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
  function initSwagger() {
    if (typeof SwaggerUIBundle !== 'undefined') {
      SwaggerUIBundle({
        url: '../swagger.yaml',
        dom_id: '#swagger-ui-container',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis
        ],
        layout: "BaseLayout"
      });
    } else {
      setTimeout(initSwagger, 100);
    }
  }
  initSwagger();
</script>

---

## Enabling the REST API

You can enable the built-in HTTP REST API server to dynamically manage backends, view status, and integrate with external tools.

To configure the API listen address, port, basic authentication, and TLS certificates, please refer to the **[Corefile Reference](configuration.md#rest-api-options)**.

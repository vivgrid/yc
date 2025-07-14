### Example

You can create a `yc.yml` file in the root of your project to configure the deployment settings.

```yaml
zipper: zipper.vivgrid.com:9000
app-secret: <APP_SECRET>
sfn-name: <your_function_name>
```

- zipper (optional): The URL of the Vivgrid Zipper Service endpoint, the default value is zipper.vivgrid.com:9000.
- app-secret: The application secret of your Vivgrid project.
- sfn-name: The name of your serverless function calling service.

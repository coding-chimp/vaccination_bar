# Vaccination Bar

Lambda function that tweets out the COVID-19 vaccination progress in Germany.
The data comes from [Impfdashboard.de](https://impfdashboard.de).

## Deployment

Create a lambda function in the AWS console and add all the needed environment
variables (`ACCESS_TOKEN`, `ACCESS_SECRET`, `API_KEY`, `API_SECRET`) needed to
connect to Twitter.

Prepare the binary and zip it:

```
GOOS=linux GOARCH=amd64 go build -o main bot.go
zip main.zip main
```

Then upload the zip to AWS.

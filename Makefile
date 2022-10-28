deploy-slash:
	gcloud functions deploy blablapoll --entry-point OnSlashCommandTrigger --runtime go116 --trigger-http --env-vars-file env.yaml --memory 128 --project itouillette-206313

deploy-actions:
	gcloud functions deploy blablapoll-actions --entry-point OnActionTrigger --runtime go116 --trigger-http --env-vars-file env.yaml --memory 128 --project itouillette-206313

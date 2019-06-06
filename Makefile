deploy-slash:
	gcloud functions deploy blablapoll --entry-point OnSlashCommandTrigger --runtime go111 --trigger-http --env-vars-file env.yaml --memory 128

deploy-actions:
	gcloud functions deploy blablapoll-actions --entry-point OnActionTrigger --runtime go111 --trigger-http --env-vars-file env.yaml --memory 128

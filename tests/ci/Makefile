.PHONY: test deploy rollback refresh deploy-no-update-service scale down delete help run-task status

export CLUSTER := ecspresso-test
export SERVICE := nginx-$(CIRCLE_BRANCH)
ECSPRESSO := ecspresso --envfile envfile

help:

test: verify deploy tasks diff revisions run-task deploy rollback refresh deploy-no-update-service scale down up delete

status:
	$(ECSPRESSO) status --events 10 --debug

register:
	$(ECSPRESSO) register --dry-run
	$(ECSPRESSO) register --debug

deploy:
	$(ECSPRESSO) deploy --dry-run
	$(ECSPRESSO) deploy --debug

deploy-no-update-service:
	$(ECSPRESSO) deploy --no-update-service --dry-run
	$(ECSPRESSO) deploy --no-update-service --force-new-deployment --debug

refresh:
	$(ECSPRESSO) refresh --dry-run
	$(ECSPRESSO) refresh --debug

rollback:
	$(ECSPRESSO) rollback --deregister-task-definition --dry-run
	$(ECSPRESSO) rollback --deregister-task-definition --debug

scale:
	$(ECSPRESSO) scale --tasks 2 --dry-run
	$(ECSPRESSO) scale --tasks 2 --debug

down:
	$(ECSPRESSO) scale --tasks 0 --dry-run
	$(ECSPRESSO) scale --tasks 0 --debug

up:
	$(ECSPRESSO) scale --tasks 1

delete:
	$(ECSPRESSO) delete --dry-run
	$(ECSPRESSO) delete --force --terminate --debug

run-task:
	$(ECSPRESSO) --envfile envfile.override run \
		--overrides '{"containerOverrides":[{"name":"nginx", "command":["nginx", "-V"]}]}' \
		--dry-run
	$(ECSPRESSO) --envfile envfile.override run \
		--overrides '{"containerOverrides":[{"name":"nginx", "command":["nginx", "-V"]}]}' \
		--debug
	OPTION=-v $(ECSPRESSO) --envfile envfile.override run \
		--overrides-file=overrides.json \
		--dry-run
	OPTION=-v $(ECSPRESSO) --envfile envfile.override run \
		--overrides-file=overrides.json \
		--wait-until=running \
		--debug
	OPTION=-v $(ECSPRESSO) --envfile envfile.override run \
		--overrides-file=overrides.json \
		--wait-until=stopped \
		--debug

wait:
	$(ECSPRESSO) wait

verify:
	$(ECSPRESSO) verify --debug

tasks:
	$(ECSPRESSO) tasks
	$(ECSPRESSO) tasks --output tsv
	$(ECSPRESSO) tasks --output json
	$(ECSPRESSO) tasks --find

exec:
	$(ECSPRESSO) exec

diff:
	$(ECSPRESSO) diff --assume-role-arn arn:aws:iam::$(AWS_ACCOUNT_ID):role/ecspresso-test

revisions:
	$(ECSPRESSO) revisions
	$(ECSPRESSO) revisions --revision=current
	$(ECSPRESSO) revisions --revision=latest

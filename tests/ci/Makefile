.PHONY: test create deploy rollback refresh deploy-update-service scale down delete help run-task status

export CLUSTER := ecspresso-test
export SERVICE := nginx-$(CIRCLE_BRANCH)

help:

test: create run-task deploy rollback refresh deploy-update-service scale down delete

status:
	ecspresso --config config.yaml status --events 10 --debug

create:
	ecspresso --config config.yaml create --dry-run
	ecspresso --config config.yaml create --debug

deploy:
	ecspresso --config config.yaml deploy --dry-run
	ecspresso --config config.yaml deploy --debug

deploy-update-service:
	ecspresso --config config.yaml deploy --update-service --dry-run
	ecspresso --config config.yaml deploy --update-service --force-new-deployment --debug

refresh:
	ecspresso --config config.yaml refresh --dry-run
	ecspresso --config config.yaml refresh --debug

rollback:
	ecspresso --config config.yaml rollback --deregister-task-definition --dry-run
	ecspresso --config config.yaml rollback --deregister-task-definition --debug

scale:
	ecspresso --config config.yaml deploy --tasks 2 --skip-task-definition --dry-run
	ecspresso --config config.yaml deploy --tasks 2 --skip-task-definition --debug

down:
	ecspresso --config config.yaml deploy --tasks 0 --skip-task-definition --dry-run
	ecspresso --config config.yaml deploy --tasks 0 --skip-task-definition --debug

delete:
	ecspresso --config config.yaml delete --dry-run
	ecspresso --config config.yaml delete --force --debug

run-task:
	ecspresso --config config.yaml run \
	--overrides '{"containerOverrides":[{"name":"nginx", "command":["nginx", "-V"]}]}'

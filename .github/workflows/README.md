# `/.github/workflows`

This is the location of the github actions workflows. Workflows are executed by github regarding your defined triggers.

For more information about github actions see [Github Actions Docs](https://docs.github.com/en/actions)

## `publish.yml`

This action pushes your Docker image to AWS elastic container registry. We are using an open source action for this.

More infos: 
- [Elastic container registry](https://aws.amazon.com/ecr)
- [Action: kciter/aws-ecr-action](https://github.com/kciter/aws-ecr-action)

## `main.yml`

This action is linting and testing your go code. We created a private action to unify the code standards for go projects.

More infos: 
- [Go Actions Repository](https://github.com/plentymarkets/actions-go)
# plentymarkets microservice go boilerplate

## Getting Started

This repository should serve as a template for future microservices.

### How do i use this boilerplate
Use the `Use this template` button on top of the page, or download the Zip file.

![Use thi template image](https://user-images.githubusercontent.com/9029015/107752501-2198ae00-6d1f-11eb-9807-93ded5471996.png)

## Directories

### `/.github`

Location of github related files. For example the github action to deploy your service is located here

See the [`/.github/workflows`](.github/workflows) directory for examples.

### `/cmd`

Main applications for this project.

The directory name for each application should match the name of the executable you want to have (e.g., `/cmd/myapp`).

Don't put a lot of code in the application directory. If you think the code can be imported and used in other projects, then it should live in the `/pkg` directory. If the code is not reusable or if you don't want others to reuse it, put that code in the `/internal` directory. You'll be surprised what others will do, so be explicit about your intentions!

It's common to have a small `main` function that imports and invokes the code from the `/pkg` directories and nothing else.

See the [`/cmd`](cmd) directory for examples.

### `/pkg`

Library code that's ok to use by external applications (e.g., `/pkg/mypubliclib`). Other projects will import these libraries expecting them to work, so think twice before you put something here. The `/pkg` directory is still a good way to explicitly communicate that the code in that directory is safe for use by others. 

See the [`/pkg`](pkg) directory for examples.

## Files

### `Dockerfile`

The dockerfile has the job to containerize youre application. This is needed to deploy it to kubernetes

### `go.mod` / `go.sum`

Go Package Manager files like `composer.json` and `composer.lock`in PHP. [More Infos](https://github.com/golang/go/wiki/Modules)

## Extended Project Structure
[https://github.com/golang-standards/project-layout](https://github.com/golang-standards/project-layout)

## Other go examples

- [AuditLogSearch](https://github.com/plentymarkets/audit-log-search)
- [Validation](https://github.com/plentymarkets/microservice-validation)
- [PlentyCountr](https://github.com/plentymarkets/plentycountr)

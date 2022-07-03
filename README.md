# Price monitoring
The test task. Creating a tiny service which allows to monitor Bitcoin prices over a specified period of time with specified frequency.

## Task
The task is it to create a tiny service which allows to monitor Bitcoin prices over a specified period of time with specified frequency. For example a user
sends a request to the service to monitor Bitcoin price for a period of 5 minutes with frequency every 10 seconds. User is assumed to be able to do 2 operations:
1. Call the service with period and frequency parameters, which will start monitoring. Returns monitoring ID. A user can start many monitors in parallel without waiting for previous monitors to finish.
2. Call the service with monitoring ID to check if monitoring is complete and to get results (if monitoring is not finished yet, return some kind of error).

Full task description:

![img](./.img/task_description.png)

## Solution


#### Main architecture diagram

![img](./.img/main_diagram.png)


I have chosen first approach.


ERP 

![img.png](.img/db.png)

## Before please install this 

1) [Docker and Docker Compose](https://docs.docker.com/compose/install/)
2) Be sure that your system support Makefile [more_info](https://askubuntu.com/questions/161104/how-do-i-install-make)
3) Optional Go (for local development) [Go install](https://go.dev/doc/install)

#### Run 

From root of the repository:

    make start 


#### Stop

    make stop


#### How to make requests

1) Health check 

     ```curl --request GET --url http://localhost:4000/health```

2) Start monitoring

    ```curl --request POST --url http://localhost:4000/api/v1/monitoring?cur=btcusd&period=1m&freq=10s```

3) Get results of monitoring 

    ```curl --request GET --url http://localhost:4000/api/v1/monitoring/1```


#### Insomnia examples:

see in folder -> `.insomnia`

![img](./.img/example_post.png)
![img](./.img/example_get.png)


#### Docker Compose run

![img](./.img/result_running_service_in_docker_compose.png)

# TODOS:

0) Refactor a little (see many todo's)
1) Full-implementing hexagonal-architecture ->  https://medium.com/@matiasvarela/hexagonal-architecture-in-go-cfd4e436faa3
2) Tests-tests-tests -> Increase test coverage (http api tests cases, consul - MUST!)
3) Add more errors handlers
4) Think about better sql select where case (select prices)
5) Add pagination
6) Add Swagger docs
7) Implements 2 and 3 variants of architecture =) 
7.1) Compare them in real world with benchmarks


## Development

#### Env up

   make dev_env_up

After that can run go app (in JetBrains IDE or VSCode) (do not forget set up .env file and add ENVs)

![img](./.img/ide_settings_1.png)
![img](./.img/ide_settings_2.png)
   
#### Remove old container or volumes

   make docker_clean_all
   
#### Rebuild prod app container

   make rebuild
   
Made by Arseny Sazanov 2022

arseny.sazanov@gmail.com

MIT

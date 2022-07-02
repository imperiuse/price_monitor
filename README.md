# Price monitoring
The test task. Creating a tiny service which allows to monitor Bitcoin prices over a specified period of time with specified frequency.

## Task
The task is it to create a tiny service which allows to monitor Bitcoin prices over a specified period of time with specified frequency. For example a user
sends a request to the service to monitor Bitcoin price for a period of 5 minutes with frequency every 10 seconds. User is assumed to be able to do 2 operations:
1. Call the service with period and frequency parameters, which will start monitoring. Returns monitoring ID. A user can start many monitors in parallel without waiting for previous monitors to finish.
2. Call the service with monitoring ID to check if monitoring is complete and to get results (if monitoring is not finished yet, return some kind of error).


## Solution


#### Main architecture diagram

![img](./.img/main_diagram.png)

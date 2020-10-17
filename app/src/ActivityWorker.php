<?php

/**
 * Spiral Framework.
 *
 * @license   MIT
 * @author    Anton Titov (Wolfy-J)
 */

declare(strict_types=1);

namespace Temporal\Client;

class ActivityWorker
{
    // Optional: To set the maximum concurrent activity executions this worker can have.
    // The zero value of this uses the default value.
    // default: defaultMaxConcurrentActivityExecutionSize(1k)
    public const MAX_CONCURRENT_ACTIVITY_EXECUTIONS_SIZE = 'maxConcurrentActivityExecutionSize';

    // Optional: Sets the rate limiting on number of activities that can be executed per second per
    // worker. This can be used to limit resources used by the worker.
    // Notice that the number is represented in float, so that you can set it to less than
    // 1 if needed. For example, set the number to 0.1 means you want your activity to be executed
    // once for every 10 seconds. This can be used to protect down stream services from flooding.
    // The zero value of this uses the default value
    // default: 100k
    public const WORKER_ACTIVITIES_PER_SECOND = 'workerActivitiesPerSecond';

    // Optional: Sets the rate limiting on number of activities that can be executed per second.
    // This is managed by the server and controls activities per second for your entire taskqueue
    // whereas WorkerActivityTasksPerSecond controls activities only per worker.
    // Notice that the number is represented in float, so that you can set it to less than
    // 1 if needed. For example, set the number to 0.1 means you want your activity to be executed
    // once for every 10 seconds. This can be used to protect down stream services from flooding.
    // The zero value of this uses the default value.
    // default: 100k
    public const TASK_QUEUE_ACTIVITIES_PER_SECOND = 'taskQueueActivitiesPerSecond';

    // Optional: Sets the maximum number of goroutines that will concurrently poll the
    // temporal-server to retrieve activity tasks. Changing this value will affect the
    // rate at which the worker is able to consume tasks from a task queue.
    // default: 2
    public const MAX_CONCURRENT_ACTIVITY_TASK_POLLERS = 'maxConcurrentActivityTaskPollers';

    // Optional: worker graceful stop timeout
    // default: 0s
    public const WORKER_STOP_TIMEOUT = 'workerStopTimeout';

    /** @var array */
    private $methods;

    /** @var array */
    private $options;

    /**
     * ActivityWorker constructor.
     *
     * @param array $options
     */
    public function __construct(array $options)
    {
        $this->options = $options;
    }

    /**
     * @return array
     */
    public function getOptions(): array
    {
        return $this->options;
    }

    /**
     * @return array
     */
    public function getActivities(): array
    {
        return array_keys($this->methods);
    }

    /**
     * @param string   $name
     * @param callable $handler
     */
    public function registerActivity(string $name, callable $handler)
    {
        $this->methods [$name] = $handler;
    }
}

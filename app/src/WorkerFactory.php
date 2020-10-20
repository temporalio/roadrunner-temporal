<?php

/**
 * Spiral Framework.
 *
 * @license   MIT
 * @author    Anton Titov (Wolfy-J)
 */

declare(strict_types=1);

namespace Temporal\Client;

use Spiral\RoadRunner\Worker;

class WorkerFactory
{
    private $worker;

    /**
     * @var ActivityWorker[]
     */
    private $activityWorkers = [];

    /**
     * @param Worker $worker
     */
    public function __construct(Worker $worker)
    {
        $this->worker = $worker;
    }

    /**
     * @param string $queue
     * @param array  $options
     * @return ActivityWorker
     */
    public function createActivityWorker(string $queue, array $options = []): ActivityWorker
    {
        $activityWorker = new ActivityWorker($options);
        $this->activityWorkers[$queue] = $activityWorker;

        return $activityWorker;
    }

    public function run()
    {
        while ($payload = $this->worker->receive($context)) {
            try {
                if ($context === null) {
                    // assert: {"command":"GetActivityWorkers"}
                    $this->returnActiveWorkers();
                    continue;
                }

                error_log($context);
                error_log($payload);

                $this->worker->send(json_encode($this->invoke(
                    json_decode($context, true),
                    json_decode($payload, true)
                )));
            } catch (\Throwable $e) {
                $this->worker->error((string) $e);
            }
        }
    }

    /**
     * @param array $context
     * @param array $payload
     * @return mixed
     */
    private function invoke(array $context, array $payload)
    {
        $worker = $this->activityWorkers[$context['TaskQueue']];

        // todo: what is array returned? wrap it somehow
        return [$worker->invoke($context, $payload)];
    }

    /**
     *
     */
    private function returnActiveWorkers()
    {
        $workers = [];

        foreach ($this->activityWorkers as $queue => $worker) {
            $workers[$queue] = [
                'options'    => $worker->getOptions(),
                'activities' => $worker->getActivities()
            ];
        }

        $this->worker->send(json_encode($workers));
    }
}

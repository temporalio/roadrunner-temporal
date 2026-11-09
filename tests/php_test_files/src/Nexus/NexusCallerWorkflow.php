<?php

declare(strict_types=1);

namespace Temporal\Tests\Nexus;

use Temporal\Workflow;
use Temporal\Workflow\NexusOperationOptions;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;

#[WorkflowInterface]
class NexusCallerWorkflow
{
    #[WorkflowMethod(name: 'NexusCallerWorkflow')]
    public function run(string $endpoint, string $name, bool $async): iterable
    {
        $greeting = Workflow::newNexusServiceStub(
            GreetingService::class,
            NexusOperationOptions::new()
                ->withEndpoint($endpoint)
                ->withScheduleToCloseTimeout(30),
        );

        return yield $async ? $greeting->greetAsync($name) : $greeting->greet($name);
    }
}

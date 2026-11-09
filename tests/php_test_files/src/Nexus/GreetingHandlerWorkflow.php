<?php

declare(strict_types=1);

namespace Temporal\Tests\Nexus;

use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;

#[WorkflowInterface]
class GreetingHandlerWorkflow
{
    #[WorkflowMethod(name: 'NexusGreetingHandlerWorkflow')]
    public function handle(string $name): iterable
    {
        yield Workflow::timer(1);

        return "Hello async, {$name}!";
    }
}

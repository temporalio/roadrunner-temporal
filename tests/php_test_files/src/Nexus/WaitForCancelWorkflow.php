<?php

declare(strict_types=1);

namespace Temporal\Tests\Nexus;

use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;

#[WorkflowInterface]
class WaitForCancelWorkflow
{
    #[WorkflowMethod(name: 'NexusWaitForCancelWorkflow')]
    public function handle(string $name): iterable
    {
        yield Workflow::await(static fn(): bool => false);

        return $name;
    }
}

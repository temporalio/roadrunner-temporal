<?php

declare(strict_types=1);

namespace Temporal\Tests\Nexus;

use Temporal\Client\WorkflowOptions;
use Temporal\Nexus\Attribute\AsyncOperation;
use Temporal\Nexus\Attribute\Operation;
use Temporal\Nexus\Attribute\Service;
use Temporal\Nexus\Exception\OperationException;
use Temporal\Nexus\Nexus;
use Temporal\Nexus\WorkflowHandle;

#[Service(name: 'GreetingService')]
class GreetingService
{
    #[Operation]
    public function greet(string $name): string
    {
        return "Hello, {$name}!";
    }

    #[AsyncOperation(output: 'string')]
    public function greetAsync(string $name): WorkflowHandle
    {
        return WorkflowHandle::fromWorkflowMethod(
            GreetingHandlerWorkflow::class,
            WorkflowOptions::new()->withWorkflowId(Nexus::getStartDetails()->requestId),
            $name,
        );
    }

    #[Operation]
    public function fail(string $name): string
    {
        throw OperationException::failed("Cannot greet {$name}");
    }

    #[AsyncOperation(output: 'string')]
    public function waitForCancel(string $name): WorkflowHandle
    {
        return WorkflowHandle::fromWorkflowMethod(
            WaitForCancelWorkflow::class,
            WorkflowOptions::new()->withWorkflowId(Nexus::getStartDetails()->requestId),
            $name,
        );
    }
}

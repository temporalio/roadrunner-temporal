<?php

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class SimpleSignaledWorkflowWithSleep
{
    private $counter = 0;

    #[Workflow\SignalMethod(name: "add")]
    public function add(
        int $value
    ) {
        $this->counter += $value;
    }

    #[WorkflowMethod(name: 'SimpleSignaledWorkflowWithSleep')]
    public function handler(): iterable
    {
        // collect signals during 5 seconds ?
        yield Workflow::timer(5);

        if (!Workflow::isReplaying()) {
            sleep(1);
        }

        return $this->counter;
    }
}

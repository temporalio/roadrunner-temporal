<?php

/**
 * Spiral Framework.
 *
 * @license   MIT
 * @author    Anton Titov (Wolfy-J)
 */

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;

class SimpleSignalledWorkflowWithSleep
{
    private $counter = 0;

    #[Workflow\SignalMethod(name: "add")]
    public function add(int $value)
    {
        $this->counter += $value;
    }

    #[WorkflowMethod(name: 'SimpleSignalledWorkflowWithSleep')]
    public function handler(): iterable
    {
        // collect signals during one second
        yield Workflow::timer(1);

        sleep(1);
        return $this->counter;
    }
}

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

class SimpleSignalledWorkflow
{
    private $counter;

    #[Workflow\SignalMethod]
    public function add()
    {
        $this->counter++;
    }

    #[WorkflowMethod(name: 'SimpleSignalledWorkflow')]
    public function handler(): iterable
    {
        // collect signals during one second
        yield Workflow::timer(1);

        return $this->counter;
    }
}

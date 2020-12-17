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

class DelayedWorkflow
{
    #[WorkflowMethod(name: 'DelayedWorkflow')]
    public function handler(): iterable
    {
        yield Workflow::timer(1);

        return 'OK';
    }
}

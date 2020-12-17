<?php

/**
 * Spiral Framework.
 *
 * @license   MIT
 * @author    Anton Titov (Wolfy-J)
 */

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Internal\Support\DateInterval;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;

class TimerWorkflow
{
    #[WorkflowMethod(name: 'TimerWorkflow')]
    public function handler(string $input): iterable
    {
        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(5)
        );

        yield Workflow::timer(1);

        return yield $simple->lower($input);
    }
}

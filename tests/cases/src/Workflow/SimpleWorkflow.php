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

class SimpleWorkflow
{
    #[WorkflowMethod(name: 'SimpleWorkflow')]
    public function handler(string $input): iterable
    {
        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()
                ->withStartToCloseTimeout(
                    DateInterval::parse(5, DateInterval::FORMAT_SECONDS)
                )
        );

        return yield $simple->echo($input);
    }
}

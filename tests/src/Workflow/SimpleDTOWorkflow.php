<?php

declare(strict_types=1);

namespace Temporal\Tests\Workflow;

use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Internal\Support\DateInterval;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;
use Temporal\Tests\DTO\Message;
use Temporal\Tests\DTO\User;

class SimpleDTOWorkflow
{
    #[WorkflowMethod(name: 'SimpleDTOWorkflow')]
    public function handler(): iterable
    {
        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()
                ->withStartToCloseTimeout(5)
        );

        $u = new User();
        $u->name = "Antony";
        $u->email = "email@domain.com";

        $value = yield $simple->greet($u);
        if (!$value instanceof Message) {
            return "FAIL";
        }

        return $value;
    }
}

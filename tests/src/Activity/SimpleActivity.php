<?php

namespace Temporal\Tests\Activity;

use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Tests\DTO\Message;
use Temporal\Tests\DTO\User;

#[ActivityInterface(prefix: "SimpleActivity.")]
class SimpleActivity
{
    #[ActivityMethod]
    public function echo(string $input): string
    {
        return strtoupper($input);
    }

    #[ActivityMethod]
    public function lower(string $input): string
    {
        return strtolower($input);
    }

    #[ActivityMethod]
    public function greet(User $user): Message
    {
        return new Message(sprintf("Hello %s <%s>", $user->name, $user->email));
    }

    #[ActivityMethod]
    public function slow(string $input): string
    {
        sleep(2);

        return strtolower($input);
    }
}
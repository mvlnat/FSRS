export function About() {
  return (
    <div className="about-page">
      <section className="about-hero">
        <p className="about-kicker">About FSRS</p>
        <h1>How Study Works</h1>
        <p className="about-lead">
          This app uses the standard FSRS scheduler with the stock defaults: short-term learning steps are enabled,
          fuzz is disabled, and card timing is driven by the saved card state rather than by the browser tab.
        </p>
      </section>

      <div className="about-grid">
        <section className="about-card">
          <h2>1. New Cards</h2>
          <p>
            A brand-new card has no saved study state yet, so it is due immediately.
          </p>
          <ul className="about-flow">
            <li><strong>Again</strong>: comes back in about 1 minute.</li>
            <li><strong>Hard</strong>: comes back in about 5 minutes.</li>
            <li><strong>Good</strong>: comes back in about 10 minutes.</li>
            <li><strong>Easy</strong>: skips the short steps and goes straight to a day-based review interval.</li>
          </ul>
        </section>

        <section className="about-card">
          <h2>2. Learning And Relearning</h2>
          <p>
            Minute-based steps live in the card&apos;s saved state. If you refresh the page or leave study and come back,
            the app rebuilds the waiting queue from the backend using the saved due timestamp.
          </p>
          <ul className="about-flow">
            <li><strong>Learning</strong>: a new card that is still in the short steps.</li>
            <li><strong>Relearning</strong>: a review card you missed and now need to re-stabilize.</li>
            <li><strong>Due now</strong>: the saved due time has passed, so the card appears again in the study queue.</li>
          </ul>
        </section>

        <section className="about-card">
          <h2>3. Review Cards</h2>
          <p>
            Once a card graduates out of the short steps, FSRS switches to day-based scheduling. The next interval is
            based on the card&apos;s stability, difficulty, and how long it has been since the last review.
          </p>
          <ul className="about-flow">
            <li><strong>Again</strong>: moves the card into relearning.</li>
            <li><strong>Hard</strong>, <strong>Good</strong>, and <strong>Easy</strong>: keep the card in review, with increasingly longer intervals.</li>
          </ul>
        </section>

        <section className="about-card">
          <h2>4. Deck Counts</h2>
          <p>
            The deck list shows four buckets:
          </p>
          <ul className="about-flow">
            <li><strong>New</strong>: cards with no study state yet.</li>
            <li><strong>Due</strong>: review cards that are due right now.</li>
            <li><strong>Learning</strong>: learning or relearning cards whose short-step timer has already reached zero.</li>
            <li><strong>Total</strong>: every card in the deck.</li>
          </ul>
          <p className="about-note">
            A card waiting on a 5 or 10 minute step will not count as due or learning until that timestamp is reached.
          </p>
        </section>

        <section className="about-card">
          <h2>5. Editing Cards</h2>
          <p>
            Editing a card&apos;s front, back, or link resets its scheduling and review history. The edited card is treated
            like a new card again so the schedule stays aligned with the current content.
          </p>
        </section>

        <section className="about-card">
          <h2>6. Why Fuzz Is Off</h2>
          <p>
            Fuzz adds a small amount of randomness to day-based review intervals so identical cards do not all land on
            the exact same future date. That can help spread workload, but it also makes the schedule less predictable.
          </p>
          <p className="about-note">
            This app keeps fuzz disabled by default so intervals are easier to reason about and debug, but you can turn
            it on per deck from the settings tab if you want a less clustered long-term schedule.
          </p>
        </section>
      </div>
    </div>
  );
}

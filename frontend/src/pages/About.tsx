export function About() {
  return (
    <div className="about-page">
      <section className="about-hero">
        <p className="about-kicker">Welcome to FSRS</p>
        <h1>Learn Smarter, Not Harder</h1>
        <p className="about-lead">
          FSRS is a flashcard app that helps you memorize anything efficiently. It uses spaced repetition to show you
          cards right before you&apos;re about to forget them, so you spend less time studying while remembering more.
        </p>
      </section>

      <div className="about-grid">
        <section className="about-card">
          <h2>What is Spaced Repetition?</h2>
          <p>
            Instead of cramming, spaced repetition spreads your reviews over time. When you first learn something,
            you review it soon. As it sticks, the app waits longer before showing it again.
          </p>
          <p className="about-note">
            This matches how your brain naturally forms long-term memories, making your study time much more effective.
          </p>
        </section>

        <section className="about-card">
          <h2>Quick Start</h2>
          <ul className="about-flow">
            <li><strong>Create a deck</strong> for a topic you want to learn.</li>
            <li><strong>Add cards</strong> with a question on the front and answer on the back.</li>
            <li><strong>Study daily</strong> and rate how well you remembered each card.</li>
            <li><strong>Trust the schedule</strong> and the app handles the rest.</li>
          </ul>
        </section>

        <section className="about-card">
          <h2>Rating Your Answers</h2>
          <p>
            After revealing an answer, rate how well you knew it:
          </p>
          <ul className="about-flow">
            <li><strong>Again</strong>: You forgot or got it wrong. You&apos;ll see it again soon.</li>
            <li><strong>Hard</strong>: You remembered, but it took effort.</li>
            <li><strong>Good</strong>: You remembered it well.</li>
            <li><strong>Easy</strong>: It was effortless. The app will wait longer before asking again.</li>
          </ul>
        </section>

        <section className="about-card">
          <h2>Understanding Your Deck</h2>
          <p>
            Your deck shows these counts:
          </p>
          <ul className="about-flow">
            <li><strong>New</strong>: Cards you haven&apos;t studied yet.</li>
            <li><strong>Due</strong>: Cards ready for review today.</li>
            <li><strong>Total</strong>: All cards in the deck.</li>
          </ul>
          <p className="about-note">
            Try to clear your due cards each day. Consistency beats long study sessions.
          </p>
        </section>

        <section className="about-card">
          <h2>Tips for Success</h2>
          <ul className="about-flow">
            <li><strong>Keep cards simple</strong>: One fact per card works best.</li>
            <li><strong>Be honest</strong>: Rate based on how you actually did, not how you wanted to do.</li>
            <li><strong>Stay consistent</strong>: A few minutes daily beats hours once a week.</li>
            <li><strong>Add images or links</strong>: Extra context can help tricky cards stick.</li>
          </ul>
        </section>

        <section className="about-card">
          <h2>How FSRS Works</h2>
          <p>
            FSRS (Free Spaced Repetition Scheduler) is a modern algorithm that calculates optimal review times based on
            your performance. It tracks each card&apos;s stability and difficulty to personalize your schedule.
          </p>
          <p className="about-note">
            New cards start with short intervals (minutes), then graduate to longer ones (days, weeks, months) as you
            prove you know them.
          </p>
        </section>
      </div>
    </div>
  );
}

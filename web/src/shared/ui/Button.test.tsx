import { render, screen } from "@testing-library/react";

import { Button } from "@/shared/ui";

describe("Button", () => {
  it("renders its label", () => {
    render(<Button>Connect</Button>);

    expect(screen.getByRole("button", { name: "Connect" })).toBeInTheDocument();
  });
});

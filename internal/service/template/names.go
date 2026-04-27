package template

import (
	"fmt"
	"math/rand"
)

var adjectives = []string{
	"admiring", "adoring", "agitated", "amazing", "angry", "awesome",
	"blissful", "bold", "brave", "busy", "calm", "charming", "cheerful",
	"clever", "cool", "compassionate", "confident", "cranky", "creative",
	"curious", "daring", "dazzling", "determined", "diligent", "dreamy",
	"eager", "ecstatic", "elegant", "eloquent", "energetic", "enigmatic",
	"epic", "exciting", "fervent", "festive", "flamboyant", "focused",
	"friendly", "frosty", "funny", "gallant", "gifted", "gracious",
	"happy", "hardcore", "heroic", "hopeful", "humble", "hungry",
	"idealistic", "imaginative", "impartial", "independent", "ingenious",
	"inquisitive", "inspiring", "intuitive", "inventive", "jolly",
	"joyful", "keen", "kind", "laughing", "legendary", "lively",
	"loving", "lucid", "magical", "methodical", "mindful", "modest",
	"musing", "mysterious", "nervous", "nimble", "noble", "nostalgic",
	"objective", "optimistic", "orderly", "peaceful", "pedantic",
	"pensive", "perceptive", "persistent", "playful", "polished",
	"practical", "precise", "quirky", "radiant", "rational", "relaxed",
	"resilient", "resourceful", "reverent", "romantic", "scholarly",
	"serene", "sharp", "silly", "sleepy", "stoic", "strategic",
	"studious", "subtle", "swift", "thoughtful", "tranquil", "trusting",
	"unruffled", "upbeat", "valiant", "vibrant", "vigilant", "vigorous",
	"visionary", "whimsical", "wise", "witty", "wizardly", "wonderful",
	"xenodochial", "youthful", "zealous", "zen",
}

var nouns = []string{
	"archimedes", "aristotle", "aryabhata", "babbage", "bardeen", "bernerslee",
	"bohr", "bose", "burnell", "cannon", "carson", "cerf", "chandrasekhar",
	"chatelet", "cohen", "curie", "darwin", "diffie", "dijkstra", "dirac",
	"einstein", "elion", "euler", "faraday", "feynman", "fermat", "fermi",
	"franklin", "galileo", "gauss", "gates", "goldberg", "goodall",
	"hamilton", "hawking", "heisenberg", "hellman", "hopper", "hypatia",
	"jackson", "jennings", "johnson", "joliot", "kalam", "kapitsa",
	"kepler", "khorana", "kilby", "knuth", "kowalevski", "lamarr",
	"lamport", "leavitt", "liskov", "lovelace", "mayer", "mccarthy",
	"meitner", "mendel", "mendeleev", "mirzakhani", "moore", "morse",
	"napier", "neumann", "newton", "nightingale", "noether", "noyce",
	"panini", "pascal", "pasteur", "payne", "perlman", "planck",
	"ramanujan", "ritchie", "robinson", "rosalind", "sammet",
	"shannon", "shockley", "sinoussi", "stallman", "stonebraker",
	"swartz", "tesla", "thompson", "torvalds", "turing", "varahamihira",
	"villani", "volta", "wilbur", "wiles", "wing", "wozniak",
	"wright", "wu", "yonath",
}

var greek = []string{
	"alpha", "beta", "gamma", "delta", "epsilon", "zeta",
	"eta", "theta", "iota", "kappa", "lambda", "mue",
	"nue", "xi", "omikron", "pi", "rho", "sigma",
	"tau", "ypsilon", "phi", "chi", "psi", "omega",
}

// GenerateUniqueName returns a random adjective_noun string.
func GenerateUniqueName() string {
	adj := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	suffix := greek[rand.Intn(len(greek))]
	return adj + "_" + noun + "_" + suffix
}

func GenerateNumberedUniqueName() string {
	return GenerateUniqueName() + fmt.Sprintf("%04d", rand.Intn(10000))
}

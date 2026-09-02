# Recompute every published value in verify/claims.tsv using Ruby's rounding.
#
# Ruby's Float#round uses round-half-up by default (unlike R's banker's
# rounding), so if all six implementations agree, the published values sit
# safely away from rounding boundaries.
#
# Run: ruby verify/claims.rb <repo root>

root = ARGV[0] || "."
lines = File.readlines(File.join(root, "verify", "claims.tsv"), encoding: "UTF-8")
header = lines.shift.strip.split("\t")
col = {}
header.each_with_index { |h, i| col[h] = i }

bad = 0
worst = 0.0

lines.each do |line|
  fields = line.strip.split("\t")
  id   = fields[col["id"]]
  rule = fields[col["rule"]]
  a    = fields[col["a"]].to_f
  b    = fields[col["b"]] == "-" ? nil : fields[col["b"]].to_f
  pub  = fields[col["published"]].to_f

  got = case rule
        when "copy"   then a
        when "round0" then a.round(0).to_f
        when "round1" then a.round(1)
        when "round2" then a.round(2)
        when "ratio0" then (a / b).round(0).to_f
        when "ratio1" then (a / b).round(1)
        when "diff4"  then (a - b).round(4)
        else puts "unknown rule: #{rule}"; nil
        end

  if got.nil? || (got - pub).abs > 1e-12
    puts "DISAGREE #{id}: rule=#{rule} published=#{pub} got=#{got}"
    bad += 1
  else
    err = (got - pub).abs
    worst = err if err > worst
  end
end

if bad > 0
  puts "Ruby: #{bad} disagreement(s)"
  exit 1
end
puts "Ruby: #{lines.size} values reproduced, largest error #{'%.1e' % worst}"
